package gitserve

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseCGIResponseMapsStatusAndHeaders(t *testing.T) {
	cgiOutput := "Status: 404 Not Found\r\nContent-Type: text/plain\r\n\r\nnot found"
	recorder := httptest.NewRecorder()

	parseCGIResponse(strings.NewReader(cgiOutput), recorder)

	if recorder.Code != 404 {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("content type = %q", got)
	}
	if recorder.Body.String() != "not found" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestParseCGIResponseDefaultsToOK(t *testing.T) {
	cgiOutput := "Content-Type: application/x-git-upload-pack-result\r\n\r\nPACKDATA"
	recorder := httptest.NewRecorder()

	parseCGIResponse(strings.NewReader(cgiOutput), recorder)

	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if recorder.Body.String() != "PACKDATA" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestParseCGIResponseHandlesEmptyOutput(t *testing.T) {
	recorder := httptest.NewRecorder()

	parseCGIResponse(strings.NewReader(""), recorder)

	if recorder.Code != 502 {
		t.Fatalf("status = %d, want 502 for empty CGI output", recorder.Code)
	}
}

func TestHandlerRejectsPathTraversal(t *testing.T) {
	handler := Handler{Root: "/tmp/repos"}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/git/alice/..%2F..%2Fetc.git/info/refs", nil)
	request.URL.Path = "/git/alice/../../etc.git/info/refs"

	handler.ServeHTTP(recorder, request)

	if recorder.Code != 400 {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}
