// Copyright (c) 2023 Girino Vey!
// This file is part of the go-cli-utility project, which is licensed under the MIT License.
// See the LICENSE file in the project root for more information.

package utils

import (
	"fmt"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

// EventSigner interface for signing events
type EventSigner interface {
	GetPublicKey() (string, error)
	Sign(event *nostr.Event) error
}

// LocalSigner implementation of EventSigner using a local private key
type LocalSigner struct {
	privateKey string
}

func NewLocalSigner(privateKey string) (*LocalSigner, error) {
	secKey, err := ConvertNIP19ToHex(privateKey)
	if err != nil {
		return nil, err
	}
	return &LocalSigner{privateKey: secKey}, nil
}

func (s *LocalSigner) GetPublicKey() (string, error) {
	return nostr.GetPublicKey(s.privateKey)
}

func (s *LocalSigner) Sign(event *nostr.Event) error {
	return event.Sign(s.privateKey)
}

// FutureSigner implementation of EventSigner for future NIP-46 support
type FutureSigner struct{}

func (s *FutureSigner) GetPublicKey() (string, error) {
	// Future implementation
	return "", nil
}

func (s *FutureSigner) Sign(event *nostr.Event) error {
	// Future implementation
	return nil
}

// ConvertNIP19ToHex converts a NIP-19 key to hex format
func ConvertNIP19ToHex(key string) (string, error) {
	if strings.HasPrefix(key, "nsec") {
		_, decoded, err := nip19.Decode(key)
		if err != nil {
			return "", fmt.Errorf("failed to decode NIP-19 key: %v", err)
		}
		return decoded.(string), nil
	}
	return key, nil
}
