package nas

// NoteStationClient provides high-level operations for the Synology Note Station API.
type NoteStationClient struct {
	auth *AuthClient
}

// NewNoteStationClient creates a new Note Station client using the given AuthClient.
// The AuthClient should already be logged in before calling note operations.
func NewNoteStationClient(authClient *AuthClient) *NoteStationClient {
	return &NoteStationClient{auth: authClient}
}
