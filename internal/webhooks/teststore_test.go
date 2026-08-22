package webhooks

import (
	"context"
	"encoding/json"
	"testing"
)

// deliveryRecord captures what RecordDelivery would persist.
type deliveryRecord struct {
	hookID int64
	event  string
	status int
}

// testStore is an in-memory Store stand-in for dispatcher tests.
type testStore struct {
	hooks      []Hook
	deliveries []deliveryRecord
}

func (s *testStore) ListHooks(_ context.Context, _ int64, eventType string) ([]Hook, error) {
	matching := []Hook{}
	for _, hook := range s.hooks {
		for _, event := range hook.Events {
			if event == eventType {
				matching = append(matching, hook)
				break
			}
		}
	}
	return matching, nil
}

func (s *testStore) RecordDelivery(_ context.Context, hookID int64, eventType string, _ []byte, status int) error {
	s.deliveries = append(s.deliveries, deliveryRecord{hookID, eventType, status})
	return nil
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
