package storage

import "strings"

import "testing"

// The layout only pays off if a single prefix listing reaches everything one
// owner owns, and if re-uploading an avatar/cover lands on the same key.
func TestObjectPaths(t *testing.T) {
	userPrefix := "users/u1/"
	for _, key := range []string{AvatarPath("u1"), SessionPhotoPath("u1", "s1")} {
		if !strings.HasPrefix(key, userPrefix) {
			t.Errorf("key %q not swept by prefix %q", key, userPrefix)
		}
	}

	// A group outlives its owner's account, so its cover must not be caught
	// by a user sweep.
	if strings.HasPrefix(CoverPath("g1"), "users/") {
		t.Errorf("cover %q sits under the user prefix", CoverPath("g1"))
	}

	// Keys carry no extension, so a format change overwrites rather than
	// orphaning the previous object.
	if strings.ContainsRune(AvatarPath("u1"), '.') || strings.ContainsRune(CoverPath("g1"), '.') {
		t.Error("overwritable keys must not carry a file extension")
	}

	// Distinct sessions must not collide.
	if SessionPhotoPath("u1", "s1") == SessionPhotoPath("u1", "s2") {
		t.Error("session photos collide")
	}
}

func TestSupportedImage(t *testing.T) {
	for _, ct := range []string{"image/jpeg", "image/png"} {
		if !SupportedImage(ct) {
			t.Errorf("%s should be accepted", ct)
		}
	}
	for _, ct := range []string{"image/gif", "application/pdf", ""} {
		if SupportedImage(ct) {
			t.Errorf("%s should be rejected", ct)
		}
	}
}
