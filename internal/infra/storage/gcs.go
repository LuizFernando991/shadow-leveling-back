package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	gcs "cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// gcsUploader uploads to a Google Cloud Storage bucket. Firebase Storage's
// default bucket is a GCS bucket, so this doubles as the Firebase backend.
// Objects are tagged with a Firebase download token and the returned URL is the
// Firebase download URL (unguessable; readable by anyone holding it). Since the
// feed endpoint is authenticated, only group members ever receive the URL.
type gcsUploader struct {
	client *gcs.Client
	bucket string
}

// NewGCSUploader builds an Uploader backed by GCS/Firebase Storage.
// saJSON is the service-account credentials JSON.
func NewGCSUploader(ctx context.Context, bucket string, saJSON []byte) (Uploader, error) {
	client, err := gcs.NewClient(ctx, option.WithCredentialsJSON(saJSON))
	if err != nil {
		return nil, fmt.Errorf("storage: new gcs client: %w", err)
	}
	return &gcsUploader{client: client, bucket: bucket}, nil
}

// NewGCSUploaderFromFields builds an Uploader from the individual service-account
// fields (rather than a JSON file), so credentials can be passed as separate env
// secrets in production. privateKey may contain literal "\n" escapes (as env vars
// commonly do); they are converted back to real newlines.
func NewGCSUploaderFromFields(ctx context.Context, bucket, projectID, clientEmail, privateKey string) (Uploader, error) {
	saJSON, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"project_id":   projectID,
		"client_email": clientEmail,
		"private_key":  strings.ReplaceAll(privateKey, `\n`, "\n"),
		"token_uri":    "https://oauth2.googleapis.com/token",
	})
	if err != nil {
		return nil, fmt.Errorf("storage: build sa json: %w", err)
	}
	return NewGCSUploader(ctx, bucket, saJSON)
}

func (u *gcsUploader) Upload(ctx context.Context, path, contentType string, r io.Reader) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}

	w := u.client.Bucket(u.bucket).Object(path).NewWriter(ctx)
	w.ContentType = contentType
	w.Metadata = map[string]string{"firebaseStorageDownloadTokens": token}
	if _, err := io.Copy(w, r); err != nil {
		_ = w.Close()
		return "", fmt.Errorf("storage: copy object: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("storage: close object: %w", err)
	}

	return fmt.Sprintf(
		"https://firebasestorage.googleapis.com/v0/b/%s/o/%s?alt=media&token=%s",
		u.bucket, url.PathEscape(path), token,
	), nil
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("storage: random token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
