// Copyright (c) 2023 Girino Vey!
// This file is part of the go-cli-utility project, which is licensed under the MIT License.
// See the LICENSE file in the project root for more information.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go-cli-utility/internal/utils"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/keyer"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

var (
	imageURLs   stringSlice
	imageFiles  stringSlice
	privateKey  = flag.String("key", "", "Private key for signing the event")
	title       = flag.String("title", "", "Title of the image")
	description = flag.String("description", "", "Description of the image")
	publishedAt = flag.String("published_at", "", "Timestamp when the image was published (unix seconds)")
	relay       = flag.String("relay", "", "Relay address or path to relays.json file")
	r           = flag.String("r", "", "Relay address or path to relays.json file (short flag)")
	blossom     = flag.String("blossom", "https://cdn.nostrcheck.me", "Base URL for the blossom server")
	diff        = flag.Int("diff", 16, "Proof of work difficulty")
	signer      nostr.Keyer
)

func init() {
	flag.Var(&imageURLs, "url", "URL of the image file (can be specified multiple times)")
	flag.Var(&imageFiles, "file", "Path to the image file (can be specified multiple times)")
}

func parseAndInitParams() {
	flag.Parse()

	if *description == "" {
		*description = ""
	}
	if *publishedAt == "" {
		*publishedAt = fmt.Sprintf("%d", time.Now().Unix())
	}

	var err error
	var ok bool
	if strings.HasPrefix(*privateKey, "nsec") {
		_, decodedKey, err := nip19.Decode(*privateKey)
		if err != nil {
			log.Fatalf("Error decoding private key: %v", err)
		}
		*privateKey, ok = decodedKey.(string)
		if !ok {
			log.Fatalf("Error asserting type of decoded private key")
		}
	}
	signer, err = keyer.NewPlainKeySigner(*privateKey)
	if err != nil {
		log.Fatalf("Error creating event signer: %v", err)
	}
}

func loadRelays(relayParam string) []string {
	var relays []string
	if strings.HasPrefix(relayParam, "ws://") || strings.HasPrefix(relayParam, "wss://") {
		relays = append(relays, relayParam)
	} else if _, err := os.Stat(relayParam); err == nil {
		relays = utils.LoadRelaysFromFile(relayParam)
	}
	return relays
}

func main() {
	parseAndInitParams()

	if len(imageURLs) == 0 && len(imageFiles) == 0 {
		log.Fatalf("At least one -url or -file must be provided")
	}

	var imetaTags [][]string

	for _, imageFile := range imageFiles {
		uploadInfo, err := utils.UploadFile(*blossom, imageFile, signer)
		if err != nil {
			log.Fatalf("Error uploading image file: %v", err)
		}
		imageURL := uploadInfo["url"].(string)
		uploadedAt, ok := uploadInfo["uploaded"].(float64)
		if ok {
			*publishedAt = fmt.Sprintf("%d", int64(uploadedAt))
		} else {
			*publishedAt = fmt.Sprintf("%d", time.Now().Unix())
		}

		imetaTags = append(imetaTags, addImageIMetaTag(imageFile, imageURL))
	}

	for _, imageURL := range imageURLs {
		imagePath, err := utils.DownloadVideo(imageURL)
		if err != nil {
			log.Fatalf("Error downloading image: %v", err)
		}
		defer os.Remove(imagePath)

		imetaTags = append(imetaTags, addImageIMetaTag(imagePath, imageURL))
	}

	// Create the NIP-68 event with the extracted image information
	event, err := createNip68Event(imetaTags, title, publishedAt, description)
	if err != nil {
		log.Fatalf("Error creating NIP-68 event: %v", err)
	}

	// Sign the event with the provided private key
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := signer.SignEvent(ctx, event); err != nil {
		log.Fatalf("Error signing event: %v", err)
	}

	// Output the event data (for demonstration purposes)
	fmt.Println("Generated Event Data:", event)

	// Transmit the event to relays if the relay flag is set
	if *relay != "" || *r != "" {
		relays := loadRelays(*relay)
		if len(relays) == 0 {
			relays = loadRelays(*r)
		}
		if len(relays) > 0 {
			utils.PublishEvent(event, signer, relays)
		} else {
			log.Fatalf("No relays found to publish the event. Relay parameter: %s", *relay)
		}
	}
}

func addImageIMetaTag(imagePath string, imageURL string) nostr.Tag {
	// ignoring fileSize
	width, height, _, fileHash, bhash, mime, err := utils.ExtractMediaInfo(imagePath, "image")
	if err != nil {
		log.Fatalf("Error extracting image information: %v", err)
	}
	tag := nostr.Tag{"imeta",
		"url " + imageURL,
		"x " + fileHash,
		fmt.Sprintf("dim %dx%d", width, height),
		"m " + mime,
		fmt.Sprintf("blurhash %s", bhash)}
	return tag
}

func createNip68Event(imetaTags [][]string, title *string, publishedAt *string, description *string) (*nostr.Event, error) {
	eventKind := 20 // Event kind for picture-first feeds

	tags := nostr.Tags{
		{"title", *title},
		{"published_at", *publishedAt},
	}

	for _, imeta := range imetaTags {
		tags = append(tags, imeta)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pubKey, err := signer.GetPublicKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("Error getting public key: %v", err)
	}

	event := nostr.Event{
		Kind:      eventKind,
		PubKey:    pubKey,
		CreatedAt: nostr.Now(),
		Tags:      tags,
		Content:   *description,
	}
	err = utils.Pow(&event, *diff)
	if err != nil {
		return nil, fmt.Errorf("Error calculating proof of work: %v", err)
	}

	return &event, nil
}
