package storage

import (
	"context"
	"io"
)

// Uploader stores an object and returns a URL that can later be used to read it.
// path is the object key within the bucket (e.g. "workout-photos/<id>.jpg").
type Uploader interface {
	Upload(ctx context.Context, path, contentType string, r io.Reader) (url string, err error)
}

// noopUploader discards the bytes and returns a deterministic fake URL.
// Used in tests (mirrors email.NewNoopSender) and as the fallback when no
// bucket is configured in local development.
type noopUploader struct{}

// NewNoopUploader returns an Uploader that stores nothing.
func NewNoopUploader() Uploader { return &noopUploader{} }

func (u *noopUploader) Upload(_ context.Context, path, _ string, r io.Reader) (string, error) {
	// Drain the reader so callers behave the same as with a real backend.
	_, _ = io.Copy(io.Discard, r)
	return "https://noop.local/" + path, nil
}

// ExtForContentType returns the file extension for a supported image type.
// ok is false for anything other than JPEG/PNG.
func ExtForContentType(contentType string) (ext string, ok bool) {
	switch contentType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	default:
		return "", false
	}
}
