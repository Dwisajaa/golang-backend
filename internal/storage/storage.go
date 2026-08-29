// Package storage abstracts private file storage (payment proofs). Services
// depend on the interface; handlers never touch the filesystem directly.
package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is returned when a stored object does not exist.
var ErrNotFound = errors.New("storage: object not found")

// ErrInvalidKey guards against path traversal (Laravel checks
// `$path !== basename($path)`).
var ErrInvalidKey = errors.New("storage: invalid key")

// Storage is the seam for private object storage.
type Storage interface {
	Save(key string, r io.Reader) error
	Exists(key string) (bool, error)
	Open(key string) (io.ReadCloser, error)
	Path(key string) (string, error)
	Delete(key string) error
}

// LocalStorage stores objects in a private directory (mirrors Laravel's
// `payment_proofs` local disk rooted at storage/app/private/payment-proofs).
type LocalStorage struct {
	root string
}

func NewLocalStorage(root string) *LocalStorage { return &LocalStorage{root: root} }

// validKey enforces flat, traversal-free keys: the key must equal its own
// basename (identical to Laravel's basename comparison).
func validKey(key string) error {
	if key == "" || key != filepath.Base(key) || strings.ContainsAny(key, `/\`) || key == "." || key == ".." {
		return ErrInvalidKey
	}
	return nil
}

func (s *LocalStorage) full(key string) (string, error) {
	if err := validKey(key); err != nil {
		return "", err
	}
	return filepath.Join(s.root, key), nil
}

func (s *LocalStorage) Save(key string, r io.Reader) error {
	path, err := s.full(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *LocalStorage) Exists(key string) (bool, error) {
	path, err := s.full(key)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *LocalStorage) Open(key string) (io.ReadCloser, error) {
	path, err := s.full(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

func (s *LocalStorage) Path(key string) (string, error) {
	path, err := s.full(key)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	return path, nil
}

func (s *LocalStorage) Delete(key string) error {
	path, err := s.full(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

var _ Storage = (*LocalStorage)(nil)
