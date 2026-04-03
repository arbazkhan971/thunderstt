package api

import (
	"net/http"
)

// modelObject represents a single model entry in the OpenAI-compatible list.
type modelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// modelListResponse is the top-level envelope for the model listing endpoint.
type modelListResponse struct {
	Object string        `json:"object"`
	Data   []modelObject `json:"data"`
}

// availableModels is the static list of models the server advertises.
var availableModels = []modelObject{
	{ID: "parakeet-tdt-0.6b-v3", Object: "model", OwnedBy: "nvidia"},
	{ID: "whisper-large-v3-turbo", Object: "model", OwnedBy: "openai"},
	{ID: "whisper-large-v3", Object: "model", OwnedBy: "openai"},
	{ID: "whisper-large-v2", Object: "model", OwnedBy: "openai"},
	{ID: "whisper-medium", Object: "model", OwnedBy: "openai"},
	{ID: "whisper-small", Object: "model", OwnedBy: "openai"},
	{ID: "whisper-base", Object: "model", OwnedBy: "openai"},
	{ID: "whisper-tiny", Object: "model", OwnedBy: "openai"},
}

// HandleListModels returns an OpenAI-compatible model list. The response
// format matches the GET /v1/models endpoint of the OpenAI API.
func (s *Server) HandleListModels(w http.ResponseWriter, r *http.Request) {
	resp := modelListResponse{
		Object: "list",
		Data:   availableModels,
	}
	WriteJSON(w, http.StatusOK, resp)
}
