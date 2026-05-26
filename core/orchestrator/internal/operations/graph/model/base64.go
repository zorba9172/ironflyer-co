package model

import "encoding/base64"

// b64Encode is a tiny wrapper so call sites don't have to import the
// stdlib package; the Bytes scalar is the only consumer.
func b64Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// b64Decode is the inverse — tolerant of unpadded input so iOS / browser
// callers that elide the trailing '=' still round-trip.
func b64Decode(s string) ([]byte, error) {
	if out, err := base64.StdEncoding.DecodeString(s); err == nil {
		return out, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}
