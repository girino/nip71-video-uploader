// Copyright (c) 2023 Girino Vey!
// This file is part of the go-cli-utility project, which is licensed under the MIT License.
// See the LICENSE file in the project root for more information.

package utils

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
	"github.com/nbd-wtf/go-nostr"
	_ "golang.org/x/image/webp"
)

// ValidateInput checks if the provided video URL, private key, title, and published_at are valid
func ValidateInput(videoURL, title, publishedAt string) error {
	if videoURL == "" {
		return errors.New("video URL cannot be empty")
	}

	if _, err := url.ParseRequestURI(videoURL); err != nil {
		return errors.New("invalid video URL")
	}

	if publishedAt == "" {
		return errors.New("published_at cannot be empty")
	}

	if _, err := strconv.ParseInt(publishedAt, 10, 64); err != nil {
		return errors.New("invalid published_at timestamp")
	}

	return nil
}

// DownloadVideo downloads the video from the given URL and returns the local file path
func DownloadVideo(videoURL string) (string, error) {
	resp, err := http.Get(videoURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("failed to download video: %s", resp.Status)
	}

	file, err := os.CreateTemp("", "video-*.mp4")
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return file.Name(), nil
}

// GetImageDimensions returns the width and height of an image file
func GetImageDimensions(filePath string) (int, int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	img, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}

	return img.Width, img.Height, nil
}

// GetVideoDimensions uses ffprobe to get the dimensions of the video
func GetVideoDimensions(filePath string) (int, int, error) {

	// Get video dimensions using ffprobe
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0", "'"+filePath+"'")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, err
	}

	dimensions := strings.TrimSpace(string(output))
	parts := strings.Split(dimensions, ",")
	if len(parts) != 2 {
		return 0, 0, errors.New("failed to get video dimensions")
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}

	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}

	return width, height, nil
}

func GetMediaDimensions(filePath string, fileType string) (int, int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	head := make([]byte, 261)
	_, err = file.Read(head)
	if err != nil {
		return 0, 0, err
	}

	kind, err := filetype.Match(head)
	if err != nil {
		return 0, 0, err
	}

	if kind == types.Unknown {
		return 0, 0, errors.New("unknown file type")
	}

	if strings.HasPrefix(kind.MIME.Value, "image") && fileType == "image" {
		return GetImageDimensions(filePath)
	} else if strings.HasPrefix(kind.MIME.Value, "video") && fileType == "video" {
		return GetVideoDimensions(filePath)
	} else {
		return 0, 0, errors.New("unsupported media type")
	}
}

func ExtractMediaInfo(imagePath string, fileType string) (int, int, string, error) {
	width, height, err := GetMediaDimensions(imagePath, fileType) // Assuming the same function can be used for images
	if err != nil {
		return 0, 0, "", err
	}

	file, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return 0, 0, "", err
	}
	imageHash := fmt.Sprintf("%x", hash.Sum(nil))
	return width, height, imageHash, nil
}

func UploadFile(server, filePath string, signer EventSigner) (map[string]interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Calculate SHA256
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}
	sha256Hash := hex.EncodeToString(hasher.Sum(nil))

	// Reset file pointer
	file.Seek(0, io.SeekStart)

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// Read file content to identify MIME type
	buf := make([]byte, 261)
	_, err = file.Read(buf)
	if err != nil {
		return nil, err
	}
	kind, err := filetype.Match(buf)
	if err != nil {
		return nil, err
	}
	mimeType := kind.MIME.Value

	// Reset file pointer to the beginning
	file.Seek(0, io.SeekStart)

	// Create authorization event
	authEventJSON, err := createAuthorizationEvent(signer, "upload", [][]string{
		{"x", sha256Hash},
		{"t", "upload"},
		{"expiration", fmt.Sprintf("%d", time.Now().Add(5*time.Minute).Unix())},
	})
	if err != nil {
		return nil, err
	}

	// Create request
	uploadURL := server + "/upload"
	req, err := http.NewRequest("PUT", uploadURL, file)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mimeType)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	req.Header.Set("Authorization", "Nostr "+authEventJSON)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusCreated &&
		resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload failed: %s", string(bodyBytes))
	}

	// Parse response
	var descriptor map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&descriptor)
	if err != nil {
		return nil, err
	}

	return descriptor, nil
}

// Add a function to create the authorization event
func createAuthorizationEvent(signer EventSigner, verb string, tags [][]string) (string, error) {
	// Create a new event
	event := nostr.Event{
		Kind:      24242,
		CreatedAt: nostr.Now(),
		Tags:      nostr.Tags{},
		Content:   "Upload file",
	}

	// Add the verb tag
	event.Tags = append(event.Tags, nostr.Tag{"t", verb})

	// Add additional tags
	for _, tag := range tags {
		event.Tags = append(event.Tags, tag)
	}

	// Set the pubkey
	pubKeyHex, err := signer.GetPublicKey()
	if err != nil {
		return "", err
	}
	event.PubKey = pubKeyHex

	// Sign the event
	err = signer.Sign(&event)
	if err != nil {
		return "", err
	}

	// Serialize the event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return "", err
	}

	encodedJSON := base64.StdEncoding.EncodeToString(eventJSON)
	return encodedJSON, nil
}

func PublishEvent(event nostr.Event, signer EventSigner, relays []string) {
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
					return signer.Sign(authEvent)
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

func LoadRelaysFromFile(filePath string) []string {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening %s: %v", filePath, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var relays []string
	err = decoder.Decode(&relays)
	if err != nil {
		log.Fatalf("Error decoding %s: %v", filePath, err)
	}
	return relays
}
