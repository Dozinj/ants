package ants

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"io"
)

var WebsocketKey = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func generateChallengeKey() (string, error) {
	p := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, p); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(p), nil
}

func encryptionkey(key string)string{
	h := sha1.New()
	h.Write([]byte(key))
	h.Write(WebsocketKey)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

