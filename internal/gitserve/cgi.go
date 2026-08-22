package gitserve

import (
	"bufio"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// parseCGIResponse reads a CGI response from r and writes it to w, mapping
// the CGI Status header onto the HTTP status code and streaming the body.
func parseCGIResponse(r io.Reader, w http.ResponseWriter) {
	buffered := bufio.NewReader(r)
	status := ""
	headers := make(http.Header)
	for {
		line, err := buffered.ReadString('\n')
		if err != nil {
			http.Error(w, "git backend returned no response", http.StatusBadGateway)
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if strings.EqualFold(key, "Status") {
			status = strings.TrimSpace(value)
			continue
		}
		headers.Add(key, strings.TrimSpace(value))
	}

	code := http.StatusOK
	if status != "" {
		fields := strings.Fields(status)
		if len(fields) > 0 {
			if parsed, err := strconv.Atoi(fields[0]); err == nil {
				code = parsed
			}
		}
	}
	for key, values := range headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(code)
	if _, err := io.Copy(w, buffered); err != nil {
		// Client went away mid-stream; nothing further to do.
		_ = err
	}
}
