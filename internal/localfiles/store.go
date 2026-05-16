package localfiles

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultTTL = time.Hour

var ErrNotFound = errors.New("local file not found")

type File struct {
	ID          string
	Filename    string
	ContentType string
	Data        []byte
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type Store struct {
	mu    sync.RWMutex
	files map[string]File
	ttl   time.Duration
	now   func() time.Time
}

var DefaultStore = NewStore(defaultTTL)

func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Store{
		files: map[string]File{},
		ttl:   ttl,
		now:   time.Now,
	}
}

func (s *Store) Put(filename, contentType string, data []byte) (File, error) {
	if s == nil {
		return File{}, errors.New("local file store is nil")
	}
	if len(data) == 0 {
		return File{}, errors.New("file is required")
	}
	id, err := newID()
	if err != nil {
		return File{}, err
	}
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "." || filename == "" {
		filename = "upload.bin"
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	now := s.now()
	file := File{
		ID:          id,
		Filename:    filename,
		ContentType: contentType,
		Data:        append([]byte(nil), data...),
		CreatedAt:   now,
		ExpiresAt:   now.Add(s.ttl),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(now)
	s.files[id] = file
	return file, nil
}

func (s *Store) Get(id string) (File, bool) {
	if s == nil {
		return File{}, false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return File{}, false
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(now)
	file, ok := s.files[id]
	if !ok {
		return File{}, false
	}
	return cloneFile(file), true
}

func (s *Store) ServeHTTP(w http.ResponseWriter, r *http.Request, id string) {
	file, ok := s.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if disposition := contentDisposition(file.Filename); disposition != "" {
		w.Header().Set("Content-Disposition", disposition)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func PublicURL(baseURL, id string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	id = strings.TrimSpace(id)
	if baseURL == "" {
		return "", errors.New("deepseek.file_base_url is required for local file URLs")
	}
	if id == "" {
		return "", errors.New("local file id is required")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("deepseek.file_base_url must be an absolute http(s) URL")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return "", errors.New("deepseek.file_base_url must use http or https")
	}
	return baseURL + "/__ds2api/files/" + url.PathEscape(id), nil
}

func IsLocalID(id string) bool {
	return strings.HasPrefix(strings.TrimSpace(id), "localfile_")
}

func cloneFile(file File) File {
	file.Data = append([]byte(nil), file.Data...)
	return file
}

func (s *Store) purgeExpiredLocked(now time.Time) {
	for id, file := range s.files {
		if !file.ExpiresAt.IsZero() && now.After(file.ExpiresAt) {
			delete(s.files, id)
		}
	}
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "localfile_" + hex.EncodeToString(b[:]), nil
}

func contentDisposition(filename string) string {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "" || filename == "." {
		return ""
	}
	return mime.FormatMediaType("inline", map[string]string{"filename": filename})
}
