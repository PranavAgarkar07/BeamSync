package beamsync

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTokenMiddlewareRejectsMissingOrInvalidToken(t *testing.T) {
	handler := tokenMiddleware("secret-token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	for _, target := range []string{"/upload", "/upload?token=wrong"} {
		req := httptest.NewRequest(http.MethodPost, target, nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s returned %d, want %d", target, rec.Code, http.StatusForbidden)
		}
	}
}

func TestTokenMiddlewareAllowsValidTokenAndOptions(t *testing.T) {
	handler := tokenMiddleware("secret-token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodPost, "/upload?token=secret-token", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("valid token status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	optionsReq := httptest.NewRequest(http.MethodOptions, "/upload", nil)
	optionsRec := httptest.NewRecorder()
	handler(optionsRec, optionsReq)
	if optionsRec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want %d", optionsRec.Code, http.StatusNoContent)
	}
}

func TestAutoRenamePathFindsNonCollidingName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "photo.png"), []byte("first"), 0644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	got := autoRenamePath(dir, "photo.png")
	want := filepath.Join(dir, "photo(1).png")

	if got != want {
		t.Fatalf("autoRenamePath = %q, want %q", got, want)
	}
}

func TestGenerateTokenReturnsHexToken(t *testing.T) {
	token := generateToken()
	if len(token) != 32 {
		t.Fatalf("token length = %d, want 32", len(token))
	}
	for _, ch := range token {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Fatalf("token contains non-hex character %q", ch)
		}
	}
}
