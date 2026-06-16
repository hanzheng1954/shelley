package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"shelley.exe.dev/db"
	"shelley.exe.dev/db/generated"
)

// draftCwdPut sends a PUT /draft-cwd for conversationID and returns the recorder.
func draftCwdPut(t *testing.T, server *Server, conversationID, cwd string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(UpdateDraftCwdRequest{Cwd: cwd})
	if err != nil {
		t.Fatalf("marshal draft-cwd request: %v", err)
	}
	httpReq := httptest.NewRequest("PUT", "/api/conversation/"+conversationID+"/draft-cwd", strings.NewReader(string(body)))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.handleUpdateDraftCwd(w, httpReq, conversationID)
	return w
}

// Changing the working directory of a draft must retarget its cwd in place
// while preserving the draft text. This backs the command-palette "set
// working directory" actions, which previously discarded the draft by
// starting a brand new conversation.
func TestUpdateDraftCwdPreservesDraftText(t *testing.T) {
	t.Parallel()
	server, database, _ := newTestServer(t)

	origCwd := "/tmp/orig"
	draft, err := database.CreateDraftConversation(context.Background(), &origCwd, nil, db.ConversationOptions{}, "my unsent draft")
	if err != nil {
		t.Fatalf("failed to create draft conversation: %v", err)
	}
	id := draft.ConversationID

	newCwd := "/tmp/newdir"
	w := draftCwdPut(t, server, id, newCwd)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := database.GetConversationByID(context.Background(), id)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if got.Cwd == nil || *got.Cwd != newCwd {
		t.Fatalf("cwd not updated: got %v, want %q", got.Cwd, newCwd)
	}
	if !got.IsDraft {
		t.Fatalf("conversation should still be a draft")
	}
	if got.Draft != "my unsent draft" {
		t.Fatalf("draft text clobbered: got %q", got.Draft)
	}

	// The response body should carry the updated conversation.
	var resp generated.Conversation
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Cwd == nil || *resp.Cwd != newCwd {
		t.Fatalf("response cwd not updated: got %v", resp.Cwd)
	}
	if resp.Draft != "my unsent draft" {
		t.Fatalf("response draft text clobbered: got %q", resp.Draft)
	}
}

// A non-draft conversation's cwd is immutable; the endpoint must 404 rather
// than silently mutating an active conversation.
func TestUpdateDraftCwdRejectsNonDraft(t *testing.T) {
	t.Parallel()
	server, database, _ := newTestServer(t)

	origCwd := "/tmp/orig"
	conv, err := database.CreateConversation(context.Background(), nil, true, &origCwd, nil, db.ConversationOptions{})
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	w := draftCwdPut(t, server, conv.ConversationID, "/tmp/newdir")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-draft, got %d: %s", w.Code, w.Body.String())
	}

	got, err := database.GetConversationByID(context.Background(), conv.ConversationID)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if got.Cwd == nil || *got.Cwd != origCwd {
		t.Fatalf("non-draft cwd should be unchanged: got %v", got.Cwd)
	}
}

// An empty cwd is a client bug; reject it rather than blanking the draft's
// working directory.
func TestUpdateDraftCwdRejectsEmpty(t *testing.T) {
	t.Parallel()
	server, database, _ := newTestServer(t)

	draft, err := database.CreateDraftConversation(context.Background(), nil, nil, db.ConversationOptions{}, "draft")
	if err != nil {
		t.Fatalf("failed to create draft conversation: %v", err)
	}

	w := draftCwdPut(t, server, draft.ConversationID, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty cwd, got %d: %s", w.Code, w.Body.String())
	}
}
