package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulsarpoint/corpscout/scheduler/internal/llm"
)

func mockServer(t *testing.T, reply string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": reply}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestComplete(t *testing.T) {
	srv := mockServer(t, "hello world")
	defer srv.Close()

	c := llm.NewClient(srv.URL, "test-model")
	result, err := c.Complete(context.Background(), "be helpful", "say hi")
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestTranslate(t *testing.T) {
	srv := mockServer(t, "Operating revenue")
	defer srv.Close()

	c := llm.NewClient(srv.URL, "test-model")
	result, err := c.Translate(context.Background(), "Driftsinntekter")
	require.NoError(t, err)
	assert.Equal(t, "Operating revenue", result)
}

func TestMaybeTranslate_SkipsASCII(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := llm.NewClient(srv.URL, "test-model")
	result := llm.MaybeTranslate(context.Background(), c, "Operating revenue")
	assert.Equal(t, "Operating revenue", result)
	assert.False(t, called, "LLM should not be called for ASCII text")
}

func TestMaybeTranslate_TranslatesNonASCII(t *testing.T) {
	srv := mockServer(t, "Operating revenue")
	defer srv.Close()

	c := llm.NewClient(srv.URL, "test-model")
	// "Bjørn Ål" has 2 non-ASCII runes out of 8 = 25% → ratio >= 0.2 → LLM is called
	result := llm.MaybeTranslate(context.Background(), c, "Bjørn Ål")
	assert.Equal(t, "Operating revenue", result)
}

func TestMaybeTranslate_LLMErrorFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := llm.NewClient(srv.URL, "test-model")
	original := "Bjørn Ål"
	result := llm.MaybeTranslate(context.Background(), c, original)
	assert.Equal(t, original, result)
}

func TestAsciiDominant(t *testing.T) {
	tests := []struct {
		text  string
		ascii bool
	}{
		{"hello world", true},
		{"Driftsinntekter", true},
		{"Bjørn Ål", false},
		{"北京", false},
	}
	for _, tc := range tests {
		var nonASCII int
		for _, r := range tc.text {
			if r > 127 {
				nonASCII++
			}
		}
		ratio := float64(nonASCII) / float64(utf8.RuneCountInString(tc.text))
		isDominant := ratio < 0.2
		assert.Equal(t, tc.ascii, isDominant, "text: %q", tc.text)
	}
}

func TestComplete_RequestShape(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "ok"}}},
		})
	}))
	defer srv.Close()

	c := llm.NewClient(srv.URL, "my-model")
	_, _ = c.Complete(context.Background(), "sys", "usr")

	assert.Equal(t, "my-model", captured["model"])
	msgs, _ := captured["messages"].([]any)
	require.Len(t, msgs, 2)
	first := msgs[0].(map[string]any)
	assert.Equal(t, "system", first["role"])
	assert.True(t, strings.Contains(first["content"].(string), "sys"))
}
