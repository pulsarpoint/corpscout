package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pulsarpoint/corpscout/scheduler/internal/httpapi"
)

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	httpapi.HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body == "" {
		t.Fatal("expected non-empty body")
	}
}
