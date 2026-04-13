package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"personal-kb/internal/nas"
	"personal-kb/internal/sync"

	"github.com/gin-gonic/gin"
)

// nasConnectRequest is the request body for POST /api/nas/connect.
type nasConnectRequest struct {
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	OTPCode  string `json:"otp_code"`
}

// NASConnect authenticates against the NAS and triggers a full sync.
func (h *Handlers) NASConnect(c *gin.Context) {
	var req nasConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	scheme := "https"
	baseURL := fmt.Sprintf("%s://%s:%d", scheme, req.Host, req.Port)
	if req.Port == 0 {
		baseURL = fmt.Sprintf("%s://%s", scheme, req.Host)
	}

	// Create a new auth client with TLS skip verify for self-signed certs
	authClient := nas.NewAuthClient(baseURL, true)

	var loginErr error
	if req.OTPCode != "" {
		loginErr = authClient.LoginWithOTP(req.Username, req.Password, req.OTPCode)
	} else {
		loginErr = authClient.Login(req.Username, req.Password)
	}

	if loginErr != nil {
		if _, ok := loginErr.(*nas.ErrOTPRequired); ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":       "2FA verification required",
				"otp_required": true,
			})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed: " + loginErr.Error()})
		return
	}

	// Store the auth client
	h.authClient = authClient
	h.nasClient = nas.NewNoteStationClient(authClient)

	// Save credentials to settings
	ctx := context.Background()
	if err := h.settingsStore.SetSetting("nas_host", req.Host); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save host: " + err.Error()})
		return
	}
	if err := h.settingsStore.SetSetting("nas_port", strconv.Itoa(req.Port)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save port: " + err.Error()})
		return
	}
	if err := h.settingsStore.SetSetting("nas_username", req.Username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save username: " + err.Error()})
		return
	}
	if err := h.settingsStore.SetSetting("nas_password_encrypted", req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save password: " + err.Error()})
		return
	}

	// Trigger full sync
	syncService := sync.NewSyncService(h.nasClient, h.notesStore)
	syncService.SetPostSyncHook(h.IndexNotesAsync)
	h.syncService = syncService

	syncedNotes := 0
	if err := syncService.FullSync(ctx); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Connected but sync had issues: " + err.Error(),
		})
		return
	}

	// Count notes
	syncedNotes, _ = h.notesStore.CountNotes(ctx, "")

	// Save last sync time
	_ = h.settingsStore.SetSetting("nas_last_sync", strconv.FormatInt(time.Now().Unix(), 10))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Connected and synced %d notes", syncedNotes),
	})
}

// NASDisconnect logs out from the NAS and clears the session.
func (h *Handlers) NASDisconnect(c *gin.Context) {
	if h.authClient != nil {
		_ = h.authClient.Logout()
	}
	h.authClient = nil
	h.nasClient = nil
	h.syncService = nil

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// NASStatus returns the current NAS connection status.
func (h *Handlers) NASStatus(c *gin.Context) {
	connected := h.authClient != nil && h.authClient.IsLoggedIn()

	host := ""
	if connected {
		host, _ = h.settingsStore.GetSetting("nas_host")
	}

	lastSync := ""
	if connected {
		lastSync, _ = h.settingsStore.GetSetting("nas_last_sync")
	}

	resp := gin.H{
		"connected": connected,
	}

	if host != "" {
		resp["host"] = host
	}
	if lastSync != "" {
		resp["last_sync"], _ = strconv.ParseInt(lastSync, 10, 64)
	}

	c.JSON(http.StatusOK, resp)
}

// NASSync triggers an incremental sync.
func (h *Handlers) NASSync(c *gin.Context) {
	if h.authClient == nil || !h.authClient.IsLoggedIn() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not connected to NAS"})
		return
	}

	if h.syncService == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sync service not initialized"})
		return
	}

	ctx := context.Background()

	beforeCount, _ := h.notesStore.CountNotes(ctx, "")

	if err := h.syncService.IncrementalSync(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync failed: " + err.Error()})
		return
	}

	afterCount, _ := h.notesStore.CountNotes(ctx, "")

	syncedNotes := afterCount - beforeCount
	if syncedNotes < 0 {
		syncedNotes = 0
	}

	_ = h.settingsStore.SetSetting("nas_last_sync", strconv.FormatInt(time.Now().Unix(), 10))

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"synced_notes": syncedNotes,
	})
}

// NASImageProxy proxies image requests to the NAS or external URLs.
// Images from NAS are cached locally so they display even without a live connection.
// Usage:
//   - GET /api/nas/image?url=<base64_url>  — proxy external image
//   - GET /api/nas/image?note_id=<id>&ref=<base64_ref>  — proxy NAS image
func (h *Handlers) NASImageProxy(c *gin.Context) {
	// External URL proxy (base64-encoded URL in "url" param)
	if encURL := c.Query("url"); encURL != "" {
		decoded, err := base64.URLEncoding.DecodeString(encURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url encoding"})
			return
		}
		h.proxyExternalImage(c, string(decoded))
		return
	}

	// NAS image proxy using /ns/dv/ URL pattern
	noteID := c.Query("note_id")
	ref := c.Query("ref")
	if noteID == "" || ref == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing note_id or ref parameter"})
		return
	}

	// Try serving from local cache first
	cachePath := imageCachePath(noteID, ref)
	if data, err := os.ReadFile(cachePath); err == nil {
		contentType := http.DetectContentType(data)
		c.Header("Content-Type", contentType)
		c.Header("Cache-Control", "public, max-age=86400")
		c.Header("X-Cache", "HIT")
		c.Data(http.StatusOK, contentType, data)
		return
	}

	if h.authClient == nil || !h.authClient.IsLoggedIn() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not connected to NAS and no cached image"})
		return
	}

	// Fetch note with attachment metadata from NAS
	note, err := h.nasClient.GetNoteWithAttachments(noteID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to get note: " + err.Error()})
		return
	}

	if note.Ver == "" || len(note.Attachments) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "note has no attachments"})
		return
	}

	// Find the attachment matching this ref
	var attKey, attName string
	for key, att := range note.Attachments {
		if att.Ref == ref {
			attKey = key
			attName = att.Name
			break
		}
	}

	if attKey == "" {
		// Ref might be the base64-encoded filename, try decoding
		if decoded, err := base64.StdEncoding.DecodeString(ref); err == nil {
			decodedName := string(decoded)
			for key, att := range note.Attachments {
				if att.Name == decodedName || att.Ref == ref {
					attKey = key
					attName = att.Name
					break
				}
			}
		}
	}

	if attKey == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "attachment not found for ref"})
		return
	}

	// Download image using /ns/dv/{link_id}/{ver}/{att_key}/{filename}
	imagePath := fmt.Sprintf("/ns/dv/%s/%s/%s/%s", note.LinkID, note.Ver, attKey, attName)
	resp, err := h.authClient.GetRaw(imagePath)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch image: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("image not available (status %d)", resp.StatusCode)})
		return
	}

	// Read image data for caching
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read image data"})
		return
	}

	// Save to local cache
	os.MkdirAll(filepath.Dir(cachePath), 0755)
	os.WriteFile(cachePath, imageData, 0644)

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(imageData)
	}
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("X-Cache", "MISS")
	c.Data(http.StatusOK, contentType, imageData)
}

// imageCachePath returns the local file path for a cached NAS image.
func imageCachePath(noteID, ref string) string {
	hash := sha256.Sum256([]byte(ref))
	filename := fmt.Sprintf("%x", hash)[:16]
	return filepath.Join("data", "images", noteID, filename)
}

// proxyExternalImage fetches an external image and serves it to the client.
func (h *Handlers) proxyExternalImage(c *gin.Context, imageURL string) {
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid image URL"})
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.toutiao.com/")
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch image"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		c.JSON(http.StatusBadGateway, gin.H{"error": "image source returned error"})
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && strings.HasPrefix(contentType, "image") {
		c.Header("Content-Type", contentType)
	} else {
		c.Header("Content-Type", "image/jpeg")
	}
	c.Header("Cache-Control", "public, max-age=86400")
	io.Copy(c.Writer, resp.Body)
}

// rewriteImageURLs replaces image src in HTML content to use the NAS image proxy.
// All images with a ref attribute are served via the proxy.
var imgTagRe = regexp.MustCompile(`<img[^>]*ref="([^"]+)"[^>]*>`)
var srcRe = regexp.MustCompile(`src="[^"]*"`)

func rewriteImageURLs(htmlContent, host, noteID string) string {
	return imgTagRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		sub := imgTagRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		ref := sub[1]
		encodedRef := url.QueryEscape(ref)
		newSrc := fmt.Sprintf(`/api/nas/image?note_id=%s&ref=%s`, url.QueryEscape(noteID), encodedRef)
		result := srcRe.ReplaceAllString(match, `src="`+newSrc+`"`)
		if result == match {
			result = strings.Replace(match, "<img ", `<img src="`+newSrc+`" `, 1)
		}
		return result
	})
}
