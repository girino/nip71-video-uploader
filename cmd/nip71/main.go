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
	hexKey      = ""
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
	hexKey, err = utils.ConvertNIP19ToHex(*privateKey)
	if err != nil {
		log.Fatalf("Error converting NIP-19 key to hex: %v", err)
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
		uploadInfo, err := utils.UploadFile(*blossom, *videoFile, hexKey)
		if err != nil {
			log.Fatalf("Error uploading video file: %v", err)
		}
		*videoURL = uploadInfo["url"].(string)
		*publishedAt = fmt.Sprintf("%d", int64(uploadInfo["uploaded"].(float64)))
		videoPath = *videoFile
	} else {
		var err error
		videoPath, err = utils.DownloadVideo(*videoURL)
		if err != nil {
			log.Fatalf("Error downloading video: %v", err)
		}
		defer os.Remove(videoPath)
	}
	// Set title to videoURL if title is unset
	if *title == "" {
		*title = *videoURL
	}

	// Validate input parameters
	if err := utils.ValidateInput(*videoURL, hexKey, *title, *publishedAt); err != nil {
		log.Fatalf("Input validation error: %v", err)
	}

	// Extract video information
	width, height, videoHash, err := utils.ExtractMediaInfo(videoPath, "video")
	if err != nil {
		log.Fatalf("Error extracting video information: %v", err)
	}

	// Create the NIP-71 event with the extracted video information
	event := createNip71Event(height, width, videoHash, title, publishedAt, videoURL, description, descriptor)

	// Sign the event with the provided private key
	if err := event.Sign(hexKey); err != nil {
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
			utils.PublishEvent(event, hexKey, relays)
		}
	}
}

func createNip71Event(height int, width int, videoHash string, title *string, publishedAt *string, videoURL *string, description *string, descriptor *string) nostr.Event {
	eventKind := 34235
	if height > width {
		eventKind = 34236
	}

	dTag := videoHash
	if *descriptor != "" {
		dTag = *descriptor
	}

	event := nostr.Event{
		Kind:      eventKind,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"d", dTag},
			{"title", *title},
			{"published_at", *publishedAt},
			{"imeta", fmt.Sprintf("dim %dx%d", width, height), "url " + *videoURL, "m video/mp4"},
		},
		Content: *description,
	}

	return event
}
