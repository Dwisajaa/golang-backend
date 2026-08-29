package storage

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func tempRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "proofs")
}

func TestSaveOpenDeleteRoundtrip(t *testing.T) {
	s := NewLocalStorage(tempRoot(t))
	content := []byte("image-bytes")
	if err := s.Save("payment-proof-abc.png", bytes.NewReader(content)); err != nil {
		t.Fatalf("save: %v", err)
	}
	ok, err := s.Exists("payment-proof-abc.png")
	if err != nil || !ok {
		t.Fatalf("exists: %v %v", ok, err)
	}
	rc, err := s.Open("payment-proof-abc.png")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(b, content) {
		t.Fatalf("content mismatch")
	}
	path, err := s.Path("payment-proof-abc.png")
	if err != nil || !strings.HasSuffix(path, "payment-proof-abc.png") {
		t.Fatalf("path: %v %v", path, err)
	}
	if err := s.Delete("payment-proof-abc.png"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if ok, _ := s.Exists("payment-proof-abc.png"); ok {
		t.Fatal("should be gone")
	}
	if _, err := s.Path("payment-proof-abc.png"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	s := NewLocalStorage(tempRoot(t))
	for _, key := range []string{"../escape.png", "a/b.png", "a\\b.png", "../../etc/passwd", "", ".", ".."} {
		if err := s.Save(key, strings.NewReader("x")); !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("key %q: expected ErrInvalidKey, got %v", key, err)
		}
	}
}

func TestSaveDoesNotOverwrite(t *testing.T) {
	s := NewLocalStorage(tempRoot(t))
	if err := s.Save("k.png", strings.NewReader("one")); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("k.png", strings.NewReader("two")); err == nil {
		t.Fatal("second save should fail (O_EXCL)")
	}
}
