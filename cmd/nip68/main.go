// Copyright (c) 2023 Girino Vey!
// This file is part of the go-cli-utility project, which is licensed under the MIT License.
// See the LICENSE file in the project root for more information.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go-cli-utility/internal/utils"

	"github.com/nbd-wtf/go-nostr"
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
	signer      utils.EventSigner
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
	signer, err = utils.NewLocalSigner(*privateKey)
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

		width, height, hash, err := utils.ExtractMediaInfo(imageFile, "image")
		if err != nil {
			log.Fatalf("Error extracting image information: %v", err)
		}
		tag := nostr.Tag{"imeta", fmt.Sprintf("dim %dx%d", width, height), fmt.Sprintf("url %s", imageURL), "m image/jpeg", fmt.Sprintf("x %s", hash)}
		imetaTags = append(imetaTags, tag)
	}

	for _, imageURL := range imageURLs {
		imagePath, err := utils.DownloadVideo(imageURL)
		if err != nil {
			log.Fatalf("Error downloading image: %v", err)
		}
		defer os.Remove(imagePath)

		width, height, hash, err := utils.ExtractMediaInfo(imagePath, "image")
		if err != nil {
			log.Fatalf("Error extracting image information: %v", err)
		}
		tag := nostr.Tag{"imeta", fmt.Sprintf("dim %dx%d", width, height), fmt.Sprintf("url %s", imageURL), "m image/jpeg", fmt.Sprintf("x %s", hash)}
		imetaTags = append(imetaTags, tag)
	}

	// Create the NIP-68 event with the extracted image information
	event := createNip68Event(imetaTags, title, publishedAt, description)

	// Sign the event with the provided private key
	if err := signer.Sign(&event); err != nil {
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
		}
	}
}

func createNip68Event(imetaTags [][]string, title *string, publishedAt *string, description *string) nostr.Event {
	eventKind := 20 // Event kind for picture-first feeds

	tags := nostr.Tags{
		{"title", *title},
		{"published_at", *publishedAt},
	}

	for _, imeta := range imetaTags {
		tags = append(tags, imeta)
	}

	event := nostr.Event{
		Kind:      eventKind,
		CreatedAt: nostr.Now(),
		Tags:      tags,
		Content:   *description,
	}

	return event
}
