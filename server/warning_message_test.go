package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"shelley.exe.dev/db"
)

func TestPredictableFailRecordsWarningMessage(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)
	h.NewConversation("fail nope", "")

	var messages []string
	waitFor(t, 5*time.Second, func() bool {
		msgs, err := h.db.ListMessages(context.Background(), h.ConversationID())
		if err != nil {
			t.Fatalf("ListMessages: %v", err)
		}
		messages = messages[:0]
		for _, msg := range msgs {
			if msg.Type != string(db.MessageTypeWarning) || msg.UserData == nil {
				continue
			}
			var userData struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal([]byte(*msg.UserData), &userData); err != nil {
				t.Fatalf("warning user_data: %v", err)
			}
			messages = append(messages, userData.Text)
		}
		return len(messages) == 1 && strings.Contains(messages[0], "nope")
	})
}
