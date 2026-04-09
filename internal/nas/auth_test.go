package nas

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewAuthClient(t *testing.T) {
	client := NewAuthClient("https://nas.example.com", true)
	if client == nil {
		t.Fatal("NewAuthClient returned nil")
	}
	if client.baseURL != "https://nas.example.com" {
		t.Errorf("baseURL = %s, want %s", client.baseURL, "https://nas.example.com")
	}
	if client.IsLoggedIn() {
		t.Error("IsLoggedIn should be false for new client")
	}
}

func TestAuthClient_Login(t *testing.T) {
	tests := []struct {
		name       string
		response   interface{}
		statusCode int
		wantErr    bool
		errMsg     string
	}{
		{
			name: "successful login",
			response: map[string]interface{}{
				"success": true,
				"data": map[string]string{
					"did": "device123",
					"sid": "session456",
				},
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "login failure - wrong password",
			response: map[string]interface{}{
				"success": false,
				"error": map[string]int{
					"code": 400,
				},
			},
			statusCode: http.StatusOK,
			wantErr:    true,
			errMsg:     "login failed",
		},
		{
			name:       "server error",
			response:   nil,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
			errMsg:     "login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				if r.URL.Path != "/webapi/entry.cgi" {
					t.Errorf("path = %s, want /webapi/entry.cgi", r.URL.Path)
				}

				// Verify form values
				if err := r.ParseForm(); err != nil {
					t.Fatalf("parse form: %v", err)
				}

				if r.Form.Get("api") != "SYNO.API.Auth" {
					t.Errorf("api = %s, want SYNO.API.Auth", r.Form.Get("api"))
				}
				if r.Form.Get("version") != "6" {
					t.Errorf("version = %s, want 6", r.Form.Get("version"))
				}
				if r.Form.Get("method") != "login" {
					t.Errorf("method = %s, want login", r.Form.Get("method"))
				}
				if r.Form.Get("account") != "testuser" {
					t.Errorf("account = %s, want testuser", r.Form.Get("account"))
				}
				if r.Form.Get("passwd") != "testpass" {
					t.Errorf("passwd = %s, want testpass", r.Form.Get("passwd"))
				}
				if r.Form.Get("session") != "NoteStation" {
					t.Errorf("session = %s, want NoteStation", r.Form.Get("session"))
				}
				if r.Form.Get("format") != "cookie" {
					t.Errorf("format = %s, want cookie", r.Form.Get("format"))
				}

				w.WriteHeader(tt.statusCode)
				if tt.response != nil {
					if err := json.NewEncoder(w).Encode(tt.response); err != nil {
						t.Fatalf("encode response: %v", err)
					}
				}
			}))
			defer server.Close()

			client := NewAuthClient(server.URL, true)
			err := client.Login("testuser", "testpass")

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error = %v, want error containing %s", err, tt.errMsg)
				}
				if client.IsLoggedIn() {
					t.Error("IsLoggedIn should be false after failed login")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if !client.IsLoggedIn() {
					t.Error("IsLoggedIn should be true after successful login")
				}
				if client.GetSessionID() != "session456" {
					t.Errorf("sessionID = %s, want session456", client.GetSessionID())
				}
			}
		})
	}
}

func TestAuthClient_Logout(t *testing.T) {
	logoutCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/webapi/entry.cgi" {
			t.Errorf("path = %s, want /webapi/entry.cgi", r.URL.Path)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		if r.Form.Get("api") == "SYNO.API.Auth" && r.Form.Get("method") == "logout" {
			logoutCalled = true
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, true)

	// Test logout without being logged in (should not error)
	err := client.Logout()
	if err != nil {
		t.Errorf("logout without session: %v", err)
	}
	if logoutCalled {
		t.Error("logout should not be called when not logged in")
	}

	// Set session manually for testing
	client.sessionID = "test-session"
	if !client.IsLoggedIn() {
		t.Error("IsLoggedIn should be true after setting session")
	}

	// Test logout when logged in
	err = client.Logout()
	if err != nil {
		t.Errorf("logout with session: %v", err)
	}
	if !logoutCalled {
		t.Error("logout should have been called")
	}
	if client.IsLoggedIn() {
		t.Error("IsLoggedIn should be false after logout")
	}
}

func TestAuthClient_TrimTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]string{
				"sid": "test123",
			},
		})
	}))
	defer server.Close()

	// Test with trailing slash
	client := NewAuthClient(server.URL+"/", true)
	err := client.Login("user", "pass")
	if err != nil {
		t.Errorf("login with trailing slash: %v", err)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
