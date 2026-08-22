package webhooks

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignAndVerify(t *testing.T) {
	payload := []byte(`{"event":"push"}`)
	signature := Sign("topsecret", payload)
	if signature == "" || len(signature) < 10 {
		t.Fatalf("signature = %q", signature)
	}
	if !Verify("topsecret", payload, signature) {
		t.Fatal("valid signature failed verification")
	}
	if Verify("wrong", payload, signature) {
		t.Fatal("wrong secret passed verification")
	}
	// Empty secret means unsigned hook; anything passes.
	if !Verify("", payload, "") {
		t.Fatal("unsigned hook should pass with empty secret")
	}
}

func TestDispatchDeliversSignedPayloadToMatchingHook(t *testing.T) {
	var gotSignature, gotEvent string
	var gotBody map[string]any
	var rawBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSignature = r.Header.Get("X-GitCastle-Signature")
		gotEvent = r.Header.Get("X-GitCastle-Event")
		rawBody, _ = io.ReadAll(r.Body)
		_ = json.Unmarshal(rawBody, &gotBody)
		w.WriteHeader(200)
	}))
	defer server.Close()

	store := &testStore{hooks: []Hook{{
		ID: 1, URL: server.URL, Secret: "hush", Events: []string{"push"},
	}}}
	dispatcher := &Dispatcher{Store: store}
	dispatcher.Dispatch(context.Background(), 7, "push", "alice/castle", "alice",
		map[string]any{"after": "abc123"})

	if gotEvent != "push" {
		t.Fatalf("event header = %q", gotEvent)
	}
	if !Verify("hush", rawBody, gotSignature) {
		t.Fatal("signature did not verify against delivered body")
	}
	if gotBody["event"] != "push" || gotBody["repository"] != "alice/castle" {
		t.Fatalf("payload = %+v", gotBody)
	}
	if len(store.deliveries) != 1 || store.deliveries[0].status != 200 {
		t.Fatalf("deliveries = %+v", store.deliveries)
	}
}

func TestDispatchSkipsNonMatchingHooks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("hook should not have been called")
	}))
	defer server.Close()

	store := &testStore{hooks: []Hook{{
		ID: 1, URL: server.URL, Secret: "", Events: []string{"issue"},
	}}}
	dispatcher := &Dispatcher{Store: store}
	dispatcher.Dispatch(context.Background(), 7, "push", "alice/castle", "alice", nil)
	if len(store.deliveries) != 0 {
		t.Fatalf("unexpected deliveries: %+v", store.deliveries)
	}
}

func TestDispatchRetriesOnFailureThenSucceeds(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	store := &testStore{hooks: []Hook{{ID: 2, URL: server.URL, Secret: "", Events: []string{"push"}}}}
	dispatcher := &Dispatcher{Store: store, MaxRetries: 5}
	dispatcher.Dispatch(context.Background(), 7, "push", "a/b", "a", nil)
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if store.deliveries[0].status != 200 {
		t.Fatalf("final status = %d", store.deliveries[0].status)
	}
}
