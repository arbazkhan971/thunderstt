package api

import (
	"net/http"
	"runtime"
)

// versionInfo holds build-time version information.
var versionInfo = struct {
	Version   string
	Commit    string
	BuildDate string
}{
	Version:   "dev",
	Commit:    "none",
	BuildDate: "unknown",
}

// SetVersionInfo is called during server startup to inject build-time version info.
func SetVersionInfo(version, commit, buildDate string) {
	versionInfo.Version = version
	versionInfo.Commit = commit
	versionInfo.BuildDate = buildDate
}

type versionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// HandleVersion returns build version information.
func (s *Server) HandleVersion(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, versionResponse{
		Version:   versionInfo.Version,
		Commit:    versionInfo.Commit,
		BuildDate: versionInfo.BuildDate,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	})
}
