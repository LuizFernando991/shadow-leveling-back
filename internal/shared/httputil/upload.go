package httputil

import (
	"mime/multipart"
	"net/http"
)

// ReadImageUpload parses a multipart request and returns the file from the
// "image" form field plus its declared content-type. It enforces maxBytes and
// writes an error response itself when something is wrong (ok=false).
// The caller owns closing the returned file.
func ReadImageUpload(w http.ResponseWriter, r *http.Request, maxBytes int64) (multipart.File, string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		Error(w, http.StatusBadRequest, "image too large or malformed multipart form")
		return nil, "", false
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		Error(w, http.StatusBadRequest, "missing image file field")
		return nil, "", false
	}
	return file, header.Header.Get("Content-Type"), true
}
