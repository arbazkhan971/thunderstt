package model

import (
	"testing"
)

func TestGetModel_known(t *testing.T) {
	tests := []struct {
		id        string
		wantName  string
		wantEng   string
		wantType  string
		wantOwner string
	}{
		{
			id:        "parakeet-tdt-0.6b-v3",
			wantName:  "Parakeet TDT 0.6B V3",
			wantEng:   "sherpa",
			wantType:  "parakeet",
			wantOwner: "NVIDIA NeMo",
		},
		{
			id:        "parakeet-tdt-0.6b-v2",
			wantName:  "Parakeet TDT 0.6B V2",
			wantEng:   "sherpa",
			wantType:  "parakeet",
			wantOwner: "NVIDIA NeMo",
		},
		{
			id:        "whisper-large-v3-turbo",
			wantName:  "Whisper Large V3 Turbo",
			wantEng:   "sherpa",
			wantType:  "whisper",
			wantOwner: "OpenAI",
		},
		{
			id:        "silero-vad",
			wantName:  "Silero VAD",
			wantEng:   "sherpa",
			wantType:  "vad",
			wantOwner: "Silero",
		},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			m, err := GetModel(tc.id)
			if err != nil {
				t.Fatalf("GetModel(%q) returned unexpected error: %v", tc.id, err)
			}
			if m.ID != tc.id {
				t.Errorf("ID = %q, want %q", m.ID, tc.id)
			}
			if m.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", m.Name, tc.wantName)
			}
			if m.Engine != tc.wantEng {
				t.Errorf("Engine = %q, want %q", m.Engine, tc.wantEng)
			}
			if m.ModelType != tc.wantType {
				t.Errorf("ModelType = %q, want %q", m.ModelType, tc.wantType)
			}
			if m.OwnedBy != tc.wantOwner {
				t.Errorf("OwnedBy = %q, want %q", m.OwnedBy, tc.wantOwner)
			}
		})
	}
}

func TestGetModel_unknown(t *testing.T) {
	_, err := GetModel("nonexistent-model")
	if err == nil {
		t.Fatal("GetModel(\"nonexistent-model\") expected error, got nil")
	}
}

func TestListModels(t *testing.T) {
	models := ListModels()
	if len(models) != 4 {
		t.Fatalf("ListModels() returned %d models, want 4", len(models))
	}

	// Verify sorted order.
	for i := 1; i < len(models); i++ {
		if models[i].ID < models[i-1].ID {
			t.Errorf("models not sorted: %q appears after %q", models[i].ID, models[i-1].ID)
		}
	}

	// Verify expected IDs are present.
	expected := []string{
		"parakeet-tdt-0.6b-v2",
		"parakeet-tdt-0.6b-v3",
		"silero-vad",
		"whisper-large-v3-turbo",
	}
	for i, want := range expected {
		if models[i].ID != want {
			t.Errorf("models[%d].ID = %q, want %q", i, models[i].ID, want)
		}
	}
}

func TestListModels_hasRequiredFields(t *testing.T) {
	models := ListModels()
	for _, m := range models {
		t.Run(m.ID, func(t *testing.T) {
			if m.ID == "" {
				t.Error("ID is empty")
			}
			if m.Name == "" {
				t.Error("Name is empty")
			}
			if m.Engine == "" {
				t.Error("Engine is empty")
			}
			if m.ModelType == "" {
				t.Error("ModelType is empty")
			}
			if m.Size == "" {
				t.Error("Size is empty")
			}
			if m.HuggingFace.Repo == "" {
				t.Error("HuggingFace.Repo is empty")
			}
			if len(m.Files) == 0 {
				t.Error("Files is empty")
			}
		})
	}
}

func TestModelInfo_languages(t *testing.T) {
	tests := []struct {
		id      string
		minLang int
		maxLang int
	}{
		{id: "parakeet-tdt-0.6b-v3", minLang: 25, maxLang: 25},
		{id: "parakeet-tdt-0.6b-v2", minLang: 1, maxLang: 1},
		{id: "whisper-large-v3-turbo", minLang: 90, maxLang: 200},
		{id: "silero-vad", minLang: 0, maxLang: 0}, // language-agnostic, nil slice
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			m, err := GetModel(tc.id)
			if err != nil {
				t.Fatalf("GetModel(%q): %v", tc.id, err)
			}
			got := len(m.Languages)
			if got < tc.minLang || got > tc.maxLang {
				t.Errorf("len(Languages) = %d, want between %d and %d", got, tc.minLang, tc.maxLang)
			}
		})
	}
}
