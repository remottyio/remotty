// Package main provides a Remotty manager server that allows terminal sessions
// to register and be accessed via web browsers through WebRTC connections.
package main

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"

	"github.com/btcsuite/btcutil/base58"
)

type SessionDescription struct {
	Sdp   string
	Key   string
	Nonce string
}

// DecodeSDP decodes a base58-encoded and zlib-compressed SDP string.
// It returns the original SDP string or an error if decoding fails.
func DecodeSDP(offer string) (string, error) {
	decodeBytes := base58.Decode(offer)
	var sd SessionDescription
	if err := json.Unmarshal(decodeBytes, &sd); err != nil {
		return "", fmt.Errorf("failed to unmarshal session description: %w", err)
	}
	var b bytes.Buffer
	b.Write(base58.Decode(sd.Sdp))
	r, err := zlib.NewReader(&b)
	if err != nil {
		return "", fmt.Errorf("failed to create zlib reader: %w", err)
	}
	defer r.Close()

	deflateBytes, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read decompressed data: %w", err)
	}
	return string(deflateBytes), nil
}

// EncodeSDP compresses an SDP string with zlib and encodes it as base58.
// It returns the encoded string or an error if encoding fails.
func EncodeSDP(sdp string) (string, error) {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	if _, err := w.Write([]byte(sdp)); err != nil {
		return "", fmt.Errorf("failed to write to zlib compressor: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("failed to close zlib compressor: %w", err)
	}

	sd := SessionDescription{
		Sdp: base58.Encode(b.Bytes()),
	}
	offerBytes, err := json.Marshal(sd)
	if err != nil {
		return "", fmt.Errorf("failed to marshal session description: %w", err)
	}
	return base58.Encode(offerBytes), nil
}
