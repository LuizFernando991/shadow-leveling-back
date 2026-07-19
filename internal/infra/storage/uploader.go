package storage

import (
	"context"
	"io"
)

// Uploader stores an object and returns a URL that can later be used to read it.
// path is the object key within the bucket (see AvatarPath and friends below).
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

// SupportedImage reports whether contentType is an image format we accept.
func SupportedImage(contentType string) bool {
	return contentType == "image/jpeg" || contentType == "image/png"
}

// Object keys are grouped by the *owner* of the resource, so everything
// belonging to one owner can be swept with a single prefix listing:
// "users/<id>/" for an account, "groups/<id>/" for a group. Group covers are
// deliberately not under users/ — a group outlives its owner's account.
//
// Keys carry no file extension: the object's Content-Type decides how it
// renders, and a fixed key means re-uploading truly overwrites instead of
// orphaning the old object when the format changes (jpeg -> png).

// AvatarPath is the key for a user's profile photo. Overwritten on each upload.
func AvatarPath(userID string) string { return "users/" + userID + "/avatar" }

// SessionPhotoPath is the key for a workout session's progress photo. Unique
// per session, so these accumulate rather than overwrite.
func SessionPhotoPath(userID, sessionID string) string {
	return "users/" + userID + "/sessions/" + sessionID
}

// CoverPath is the key for a group's cover image. Overwritten on each upload.
func CoverPath(groupID string) string { return "groups/" + groupID + "/cover" }
