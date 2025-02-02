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

var (
	videoURL    = flag.String("url", "", "URL of the video file")
	videoFile   = flag.String("file", "", "Path to the video file")
	privateKey  = flag.String("key", "", "Private key for signing the event")
	title       = flag.String("title", "", "Title of the video")
	description = flag.String("description", "", "Description of the video")
	publishedAt = flag.String("published_at", "", "Timestamp when the video was published (unix seconds)")
	relay       = flag.String("relay", "", "Relay address or path to relays.json file")
	r           = flag.String("r", "", "Relay address or path to relays.json file (short flag)")
	descriptor  = flag.String("descriptor", "", "Descriptor for the 'd' tag")
	blossom     = flag.String("blossom", "https://cdn.nostrcheck.me", "Base URL for the blossom server")
	diff        = flag.Int("diff", 16, "Proof of work difficulty")
	isLegacy    = flag.Bool("legacy", false, "Use legacy event kind")
	signer      nostr.Keyer
)

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
		log.Fatalf("Error creating signer: %v", err)
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

	if *videoURL == "" && *videoFile == "" {
		log.Fatalf("Either -url or -file must be provided")
	}

	var videoPath string
	if *videoFile != "" {
		uploadInfo, err := utils.UploadFile(*blossom, *videoFile, signer)
		if err != nil {
			log.Fatalf("Error uploading video file: %v", err)
		}
		*videoURL = uploadInfo["url"].(string)
		uploadedAt, ok := uploadInfo["uploaded"].(float64)
		if ok {
			*publishedAt = fmt.Sprintf("%d", int64(uploadedAt))
		} else {
			*publishedAt = fmt.Sprintf("%d", time.Now().Unix())
		}
		videoPath = *videoFile
	} else {
		var err error
		videoPath, err = utils.DownloadVideo(*videoURL)
		if err != nil {
			log.Fatalf("Error downloading video: %v", err)
		}
		defer os.Remove(videoPath)
	}

	// Validate input parameters
	if err := utils.ValidateInput(*videoURL, *title, *publishedAt); err != nil {
		log.Fatalf("Input validation error: %v", err)
	}

	// Extract video information
	width, height, fileSize, videoHash, bhash, mime, err := utils.ExtractMediaInfo(videoPath, "video")
	if err != nil {
		log.Fatalf("Error extracting video information: %v", err)
	}

	// Create the NIP-71 event with the extracted video information
	event, err := createNip71Event(height, width, fileSize, videoHash, bhash, mime, title, publishedAt, videoURL, description, descriptor)
	if err != nil {
		log.Fatalf("Error creating NIP-71 event: %v", err)
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
	var relays []string
	if *relay != "" || *r != "" {
		relays = loadRelays(*relay)
		if len(relays) == 0 {
			relays = loadRelays(*r)
		}
		if len(relays) > 0 {
			utils.PublishEvent(event, signer, relays)
		}
	}

}

func createNip71Event(height int, width int, fileSize int64, videoHash string, bhash string, mime string, title *string, publishedAt *string, videoURL *string, description *string, descriptor *string) (*nostr.Event, error) {
	var eventKind int
	if *isLegacy {
		eventKind = 34235
	} else {
		eventKind = 21
	}
	alt := "Horizontal Video"
	if height > width {
		eventKind += 1 // 22 or 34236
		alt = "Vertical Video"
	}

	dTag := videoHash
	if *descriptor != "" {
		dTag = *descriptor
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
		Tags: nostr.Tags{
			{"alt", alt},
			{"title", *title},
			{"published_at", *publishedAt},
			{"imeta",
				"url " + *videoURL,
				"m " + mime,
				"alt " + alt,
				"x " + videoHash,
				fmt.Sprintf("size %d", fileSize),
				fmt.Sprintf("dim %dx%d", width, height),
				fmt.Sprintf("blurhash %s", bhash)},
		},
		Content: *description,
	}
	if *isLegacy {
		event.Tags = append(event.Tags, nostr.Tag{"d", dTag})
	}

	err = utils.Pow(&event, *diff)
	if err != nil {
		return nil, fmt.Errorf("Error calculating proof of work: %v", err)
	}

	return &event, nil
}
