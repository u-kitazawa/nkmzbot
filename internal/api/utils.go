package api

import (
	"crypto/rand"
	"encoding/base64"
)

func generateRandomString(length int) string {
	// Calculate bytes needed to get desired base64 length
	// base64 encoding increases size by ~4/3, so we need fewer input bytes
	byteLength := (length * 3) / 4
	if byteLength < length {
		byteLength = length
	}

	b := make([]byte, byteLength)
	rand.Read(b)
	encoded := base64.URLEncoding.EncodeToString(b)
	if len(encoded) > length {
		return encoded[:length]
	}
	return encoded
}
