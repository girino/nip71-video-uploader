package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"go-cli-utility/internal/utils"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var (
	videoURL    = flag.String("url", "", "URL of the video file")
	videoFile   = flag.String("file", "", "Path to the video file")
	privateKey  = flag.String("key", "", "Private key for signing the event")
	title       = flag.String("title", "", "Title of the video")
	description = flag.String("description", "", "Description of the video")
	publishedAt = flag.String("published_at", "", "Timestamp when the video was published (unix seconds)")
	relay       = flag.Bool("relay", false, "Transmit the event to relays")
	r           = flag.Bool("r", false, "Transmit the event to relays (short flag)")
	descriptor  = flag.String("descriptor", "", "Descriptor for the 'd' tag")
	blossom     = flag.String("blossom", "https://haven.girino.org", "Base URL for the blossom server")
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

	hexKey = *privateKey
	if strings.HasPrefix(*privateKey, "nsec") {
		_, decodedKey, err := nip19.Decode(*privateKey)
		if err != nil {
			log.Fatalf("Error decoding nsec key: %v", err)
		}
		hexKey = decodedKey.(string)
	}
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
	width, height, videoHash := extractVideoInfo(videoPath)

	// Create the NIP-71 event with the extracted video information
	event := createNip71Event(height, width, videoHash, title, publishedAt, videoURL, description, descriptor)

	// Sign the event with the provided private key
	if err := event.Sign(hexKey); err != nil {
		log.Fatalf("Error signing event: %v", err)
	}

	// Output the event data (for demonstration purposes)
	fmt.Println("Generated Event Data:", event)

	// Transmit the event to relays if the relay flag is set
	if *relay || *r {
		publishEvent(event, hexKey)
	}
}

func extractVideoInfo(videoPath string) (int, int, string) {
	width, height, err := utils.GetVideoDimensions(videoPath)
	if err != nil {
		log.Fatalf("Error getting video dimensions: %v", err)
	}

	file, err := os.Open(videoPath)
	if err != nil {
		log.Fatalf("Error opening video file: %v", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		log.Fatalf("Error hashing video file: %v", err)
	}
	videoHash := fmt.Sprintf("%x", hash.Sum(nil))
	return width, height, videoHash
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

var relays = []string{
	"wss://relay.primal.net",
	"wss://wot.girino.org",
	"wss://nostr.girino.org",
	"wss://haven.girino.org/outbox",
	"wss://haven.girino.org/private",
}

func publishEvent(event nostr.Event, hexKey string) {
	for _, relayURL := range relays {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		relay, err := nostr.RelayConnect(ctx, relayURL)
		if err != nil {
			log.Printf("Error connecting to relay %s: %v", relayURL, err)
			continue
		}
		defer relay.Close()

		err = relay.Publish(ctx, event)
		if err != nil {
			if strings.HasPrefix(err.Error(), "msg: auth-required:") {

				authErr := relay.Auth(ctx, func(authEvent *nostr.Event) error {
					return authEvent.Sign(hexKey)
				})
				if authErr != nil {
					log.Printf("Error sending auth event to relay %s: %v", relayURL, authErr)
					continue
				}

				err = relay.Publish(ctx, event)
				if err != nil {
					log.Printf("Error publishing event to relay %s after auth: %v", relayURL, err)
				} else {
					fmt.Printf("Published event to relay %s successfully after auth\n", relayURL)
				}
			} else {
				log.Printf("Error publishing event to relay %s: %v", relayURL, err)
			}
		} else {
			fmt.Printf("Published event to relay %s successfully\n", relayURL)
		}
	}
}
