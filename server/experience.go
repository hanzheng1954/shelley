package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"shelley.exe.dev/claudetool"
	"shelley.exe.dev/db"
)

func experienceLimit(r *http.Request) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		return 50
	}
	return limit
}

func writeExperienceJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func decodeExperienceJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func (s *Server) handleListExperienceMemories(w http.ResponseWriter, r *http.Request) {
	project := claudetool.ProjectRoot(r.URL.Query().Get("cwd"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	var memories []db.MemoryItem
	var err error
	if query == "" {
		memories, err = s.db.ListMemories(r.Context(), project, experienceLimit(r))
	} else {
		memories, err = s.db.SearchMemories(r.Context(), project, query, experienceLimit(r))
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if memories == nil {
		memories = []db.MemoryItem{}
	}
	writeExperienceJSON(w, memories)
}

func (s *Server) handleCreateExperienceMemory(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Cwd            string  `json:"cwd"`
		ConversationID string  `json:"conversation_id"`
		Kind           string  `json:"kind"`
		Title          string  `json:"title"`
		Content        string  `json:"content"`
		Confidence     float64 `json:"confidence"`
	}
	if !decodeExperienceJSON(w, r, &input) {
		return
	}
	if input.Confidence == 0 {
		input.Confidence = 1
	}
	if err := claudetool.ValidateMemory("project", input.Kind, input.Title, input.Content, input.Confidence); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	memory, err := s.db.SaveMemory(r.Context(), input.ConversationID, db.MemoryDraft{
		Scope: "project", ProjectPath: claudetool.ProjectRoot(input.Cwd), Kind: input.Kind,
		Title: input.Title, Content: input.Content, Confidence: input.Confidence,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeExperienceJSON(w, memory)
}

// experienceMemoryTarget returns the memory ID and project root named by the
// request, or writes a 400 and reports false.
func experienceMemoryTarget(w http.ResponseWriter, r *http.Request) (id, project string, ok bool) {
	id = r.PathValue("id")
	cwd := r.URL.Query().Get("cwd")
	if id == "" || cwd == "" {
		http.Error(w, "memory id and cwd are required", http.StatusBadRequest)
		return "", "", false
	}
	return id, claudetool.ProjectRoot(cwd), true
}

func writeExperienceMemoryError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "memory not found", http.StatusNotFound)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (s *Server) handleUpdateExperienceMemory(w http.ResponseWriter, r *http.Request) {
	id, project, ok := experienceMemoryTarget(w, r)
	if !ok {
		return
	}
	var input struct {
		Kind       string  `json:"kind"`
		Title      string  `json:"title"`
		Content    string  `json:"content"`
		Confidence float64 `json:"confidence"`
	}
	if !decodeExperienceJSON(w, r, &input) {
		return
	}
	if err := claudetool.ValidateMemory("project", input.Kind, input.Title, input.Content, input.Confidence); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.db.UpdateMemory(r.Context(), id, project, input.Kind, input.Title, input.Content, input.Confidence); err != nil {
		writeExperienceMemoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteExperienceMemory(w http.ResponseWriter, r *http.Request) {
	id, project, ok := experienceMemoryTarget(w, r)
	if !ok {
		return
	}
	if err := s.db.DeleteMemory(r.Context(), id, project); err != nil {
		writeExperienceMemoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListExperienceJournal(w http.ResponseWriter, r *http.Request) {
	conversationID := r.URL.Query().Get("conversation_id")
	if conversationID == "" {
		http.Error(w, "conversation_id is required", http.StatusBadRequest)
		return
	}
	checkpoints, err := s.db.ListTaskCheckpoints(r.Context(), conversationID, experienceLimit(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if checkpoints == nil {
		checkpoints = []db.TaskCheckpoint{}
	}
	writeExperienceJSON(w, checkpoints)
}

func (s *Server) handleCreateExperienceCheckpoint(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ConversationID string         `json:"conversation_id"`
		Summary        string         `json:"summary"`
		State          map[string]any `json:"state"`
	}
	if !decodeExperienceJSON(w, r, &input) {
		return
	}
	if input.State == nil {
		http.Error(w, "state must be a JSON object", http.StatusBadRequest)
		return
	}
	state, err := json.Marshal(input.State)
	if err != nil {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	checkpoint, err := s.db.AppendTaskCheckpoint(r.Context(), input.ConversationID, "manual", input.Summary, state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeExperienceJSON(w, checkpoint)
}

func (s *Server) handleListExperienceDreams(w http.ResponseWriter, r *http.Request) {
	project := ""
	if cwd := r.URL.Query().Get("cwd"); cwd != "" {
		project = claudetool.ProjectRoot(cwd)
	}
	runs, err := s.db.ListDreamRuns(r.Context(), project, r.URL.Query().Get("conversation_id"), experienceLimit(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if runs == nil {
		runs = []db.DreamRun{}
	}
	writeExperienceJSON(w, runs)
}
