package nas

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AuthClient handles authentication against the Synology NAS API.
type AuthClient struct {
	baseURL    string
	httpClient *http.Client
	sessionID  string
	account    string
	password   string
}

// NewAuthClient creates a new AuthClient for the given NAS base URL.
// If insecureSkipVerify is true, TLS certificate verification is disabled
// (useful for self-signed certificates on home NAS devices).
func NewAuthClient(baseURL string, insecureSkipVerify bool) *AuthClient {
	transport := &http.Transport{}
	if insecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // opt-in for self-signed certs
		}
	}
	return &AuthClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Transport: transport,
		},
	}
}

// Login authenticates against the Synology NAS using SYNO.API.Auth.
// On success the session ID is stored internally and used for subsequent requests.
func (a *AuthClient) Login(account, password string) error {
	a.account = account
	a.password = password

	params := url.Values{}
	params.Set("api", "SYNO.API.Auth")
	params.Set("version", "6")
	params.Set("method", "login")
	params.Set("account", account)
	params.Set("passwd", password)
	params.Set("session", "NoteStation")
	params.Set("format", "cookie")

	resp, err := a.post(params)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	var result synoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("login failed: %s", synoErrorMessage(result.Error))
	}

	var data authData
	if err := json.Unmarshal(result.Data, &data); err != nil {
		return fmt.Errorf("parse login data: %w", err)
	}

	a.sessionID = data.SID
	return nil
}

// Logout ends the current session on the Synology NAS.
func (a *AuthClient) Logout() error {
	if a.sessionID == "" {
		return nil
	}

	params := url.Values{}
	params.Set("api", "SYNO.API.Auth")
	params.Set("version", "6")
	params.Set("method", "logout")
	params.Set("session", "NoteStation")

	resp, err := a.post(params)
	if err != nil {
		return fmt.Errorf("logout request: %w", err)
	}
	defer resp.Body.Close()

	a.sessionID = ""
	return nil
}

// IsLoggedIn returns true if a session is currently active.
func (a *AuthClient) IsLoggedIn() bool {
	return a.sessionID != ""
}

// GetSessionID returns the current session ID (empty string if not logged in).
func (a *AuthClient) GetSessionID() string {
	return a.sessionID
}

// post sends a POST request with the given form parameters.
// It adds the session cookie if logged in.
func (a *AuthClient) post(params url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", a.baseURL+"/webapi/entry.cgi", strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if a.sessionID != "" {
		req.AddCookie(&http.Cookie{
			Name:  "id",
			Value: a.sessionID,
		})
	}

	return a.httpClient.Do(req)
}

// get sends a GET request with the given query parameters.
// It adds the session cookie if logged in.
func (a *AuthClient) get(params url.Values) (*http.Response, error) {
	req, err := http.NewRequest("GET", a.baseURL+"/webapi/entry.cgi", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.URL.RawQuery = params.Encode()

	if a.sessionID != "" {
		req.AddCookie(&http.Cookie{
			Name:  "id",
			Value: a.sessionID,
		})
	}

	return a.httpClient.Do(req)
}

// readBody reads and closes the response body, returning its bytes.
func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// synoErrorMessage returns a human-readable message for a Synology API error.
func synoErrorMessage(err *synoError) string {
	if err == nil {
		return "unknown error"
	}
	return fmt.Sprintf("error code %d", err.Code)
}
