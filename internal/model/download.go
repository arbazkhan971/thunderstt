package model

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	// hfResolveBase is the HuggingFace Hub URL template for resolving files.
	hfResolveBase = "https://huggingface.co/%s/resolve/main/%s"

	// downloadBufSize is the buffer size used when copying download streams.
	downloadBufSize = 256 * 1024 // 256 KB
)

// DownloadModel downloads all required files for the given model ID into
// destDir. The download is atomic: files are first written to a temporary
// directory adjacent to destDir, then renamed into place on success. If a
// file already exists at the destination with the correct size it is skipped
// (resume support).
func DownloadModel(modelID string, destDir string) error {
	info, err := GetModel(modelID)
	if err != nil {
		return err
	}

	// Ensure the parent of destDir exists so we can create the temp directory
	// next to it.
	parentDir := filepath.Dir(destDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("model: create parent dir: %w", err)
	}

	// Use a temporary directory next to the final destination for atomicity.
	tmpDir, err := os.MkdirTemp(parentDir, ".dl-"+modelID+"-")
	if err != nil {
		return fmt.Errorf("model: create temp dir: %w", err)
	}
	// Clean up the temp directory on failure; on success it will have been
	// renamed away and RemoveAll will be a no-op.
	defer os.RemoveAll(tmpDir)

	client := &http.Client{
		Timeout: 0, // no overall timeout; files can be very large
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 15 * time.Second,
		},
	}

	for _, file := range info.HuggingFace.Files {
		destPath := filepath.Join(destDir, file)

		// Check whether the file already exists at the final destination with
		// the expected size (resume support).
		remoteSize, err := headFileSize(client, info.HuggingFace.Repo, file)
		if err != nil {
			// Non-fatal: we just skip the size check and re-download.
			remoteSize = -1
		}

		if remoteSize > 0 {
			if st, statErr := os.Stat(destPath); statErr == nil && st.Size() == remoteSize {
				fmt.Printf("  [skip] %s (already exists, %s)\n", file, humanBytes(remoteSize))
				// Copy to tmpDir as well so the final rename has the file.
				if err := copyFile(destPath, filepath.Join(tmpDir, file)); err != nil {
					return fmt.Errorf("model: copy cached file: %w", err)
				}
				continue
			}
		}

		tmpPath := filepath.Join(tmpDir, file)

		// Ensure subdirectories inside the temp dir exist (in case file
		// contains a path separator).
		if err := os.MkdirAll(filepath.Dir(tmpPath), 0o755); err != nil {
			return fmt.Errorf("model: create subdir: %w", err)
		}

		if err := downloadFile(client, info.HuggingFace.Repo, file, tmpPath); err != nil {
			return fmt.Errorf("model: download %s: %w", file, err)
		}
	}

	// Atomic swap: remove existing destination and rename temp into place.
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("model: remove old dir: %w", err)
	}
	if err := os.Rename(tmpDir, destDir); err != nil {
		return fmt.Errorf("model: rename into place: %w", err)
	}

	return nil
}

// headFileSize issues an HTTP HEAD request to determine the content-length of
// a file on HuggingFace Hub. Returns -1 if the size cannot be determined.
func headFileSize(client *http.Client, repo, file string) (int64, error) {
	url := fmt.Sprintf(hfResolveBase, repo, file)
	resp, err := client.Head(url)
	if err != nil {
		return -1, err
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("HEAD %s: status %d", url, resp.StatusCode)
	}

	cl := resp.Header.Get("Content-Length")
	if cl == "" {
		return -1, nil
	}
	size, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return -1, err
	}
	return size, nil
}

// downloadFile downloads a single file from HuggingFace Hub and writes it to
// destPath, printing progress to stdout.
func downloadFile(client *http.Client, repo, file, destPath string) error {
	url := fmt.Sprintf(hfResolveBase, repo, file)

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}

	totalSize := resp.ContentLength // -1 if unknown

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", destPath, err)
	}
	defer out.Close()

	pw := &progressWriter{
		fileName:   file,
		totalBytes: totalSize,
	}

	buf := make([]byte, downloadBufSize)
	_, err = io.CopyBuffer(out, io.TeeReader(resp.Body, pw), buf)
	if err != nil {
		return fmt.Errorf("write %s: %w", destPath, err)
	}

	// Final progress line.
	pw.finish()

	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", destPath, err)
	}
	return nil
}

// copyFile copies src to dst using a simple read-write.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// -------------------------------------------------------------------------
// Progress reporting
// -------------------------------------------------------------------------

// progressWriter tracks bytes written and prints periodic progress updates.
type progressWriter struct {
	fileName    string
	totalBytes  int64
	written     int64
	lastPrinted time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)

	// Throttle output to at most once every 500ms to avoid flooding the
	// terminal on fast connections.
	now := time.Now()
	if now.Sub(pw.lastPrinted) >= 500*time.Millisecond {
		pw.printProgress()
		pw.lastPrinted = now
	}
	return n, nil
}

func (pw *progressWriter) printProgress() {
	if pw.totalBytes > 0 {
		pct := float64(pw.written) / float64(pw.totalBytes) * 100
		fmt.Printf("\r  [download] %s: %s / %s (%.1f%%)",
			pw.fileName,
			humanBytes(pw.written),
			humanBytes(pw.totalBytes),
			pct,
		)
	} else {
		fmt.Printf("\r  [download] %s: %s",
			pw.fileName,
			humanBytes(pw.written),
		)
	}
}

func (pw *progressWriter) finish() {
	if pw.totalBytes > 0 {
		fmt.Printf("\r  [download] %s: %s / %s (100.0%%)\n",
			pw.fileName,
			humanBytes(pw.totalBytes),
			humanBytes(pw.totalBytes),
		)
	} else {
		fmt.Printf("\r  [download] %s: %s (done)\n",
			pw.fileName,
			humanBytes(pw.written),
		)
	}
}

// humanBytes formats a byte count into a human-readable string.
func humanBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
