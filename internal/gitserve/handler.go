// Package gitserve exposes bare repositories over Git's smart HTTP protocol
// by delegating to `git http-backend`, the reference CGI implementation.
package gitserve

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// Handler serves everything under root using git http-backend.
type Handler struct {
	Root   string
	Prefix string // URL prefix the handler is mounted at, e.g. "/git"
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Normalize /owner/name.git and /owner/name.git/... paths, allowing
	// clone URLs that omit the .git suffix.
	pathInfo := strings.TrimPrefix(r.URL.Path, h.Prefix)
	pathInfo = strings.TrimLeft(pathInfo, "/")
	if !strings.Contains(pathInfo, ".git/") && !strings.HasSuffix(pathInfo, ".git") {
		parts := strings.SplitN(pathInfo, "/", 3)
		if len(parts) >= 2 {
			parts[1] += ".git"
			pathInfo = strings.Join(parts, "/")
		}
	}
	if strings.Contains(pathInfo, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	env := append(
		systemEnviron(),
		"GIT_PROJECT_ROOT="+h.Root,
		"GIT_HTTP_EXPORT_ALL=1",
		"PATH_INFO=/"+pathInfo,
		"REQUEST_METHOD="+r.Method,
		"QUERY_STRING="+r.URL.RawQuery,
		"REMOTE_ADDR="+r.RemoteAddr,
	)
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		env = append(env, "CONTENT_TYPE="+contentType)
	}
	if encoding := r.Header.Get("Content-Encoding"); encoding != "" {
		env = append(env, "HTTP_CONTENT_ENCODING="+encoding)
	}
	if length := r.Header.Get("Content-Length"); length != "" {
		env = append(env, "CONTENT_LENGTH="+length)
	}
	if user, _, ok := r.BasicAuth(); ok {
		env = append(env, "REMOTE_USER="+user)
	}

	cmd := exec.Command("git", "http-backend")
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		http.Error(w, "backend unavailable", http.StatusInternalServerError)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "backend unavailable", http.StatusInternalServerError)
		return
	}
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("start git http-backend: %v", err), http.StatusInternalServerError)
		return
	}

	writeErr := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(stdin, r.Body)
		_ = stdin.Close()
		writeErr <- copyErr
	}()

	parseCGIResponse(stdout, w)

	if err := <-writeErr; err != nil {
		w.Header().Set("Connection", "close")
	}
	_ = stdout.Close()
	if err := cmd.Wait(); err != nil {
		// http-backend exits non-zero on protocol errors like bad packs;
		// headers are already sent, so there is nothing left to report.
		_ = err
	}
}

// systemEnviron returns a minimal environment for git http-backend.
func systemEnviron() []string {
	var env []string
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		switch key {
		case "PATH", "HOME", "TMPDIR", "LANG", "LC_ALL":
			env = append(env, entry)
		}
	}
	return env
}
