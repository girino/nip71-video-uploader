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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/buckket/go-blurhash"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip13"
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
func GetImageDimensions(filePath string) (int, int, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0, "", err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return 0, 0, "", err
	}

	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	// Generate the BlurHash string
	bhash, err := generateBlurhash(img)
	if err != nil {
		return 0, 0, "", err
	}

	return width, height, bhash, nil
}

// GetVideoDimensions uses ffprobe to get the dimensions of the video
func GetVideoDimensions(filePath string) (int, int, string, error) {

	framePath := filepath.Join(os.TempDir(), "frame.jpg")
	cmd := exec.Command("ffmpeg", "-i", filePath, "-ss", "00:00:01.000", "-vframes", "1", framePath)
	if err := cmd.Run(); err != nil {
		return 0, 0, "", fmt.Errorf("extracting frame from video: %v", err)
	}
	defer os.Remove(framePath)

	return GetImageDimensions(framePath)
}

func generateBlurhash(img image.Image) (string, error) {
	x, y := 9, 7
	if img.Bounds().Dx() < img.Bounds().Dy() {
		x, y = y, x
	}

	// Generate the BlurHash string
	bhash, err := blurhash.Encode(x, y, img)
	if err != nil {
		return "", fmt.Errorf("generating blurhash: %v", err)
	}
	return bhash, nil
}

func GetMediaDimensions(filePath string, fileType string) (int, int, string, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	head := make([]byte, 261)
	_, err = file.Read(head)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("error reading file: %v", err)
	}

	kind, err := filetype.Match(head)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("error matching file: %v", err)
	}

	if kind == types.Unknown {
		return 0, 0, "", "", errors.New("unknown file type")
	}
	file.Close()

	if strings.HasPrefix(kind.MIME.Value, "image") && fileType == "image" {
		width, height, bhash, nil := GetImageDimensions(filePath)
		return width, height, bhash, kind.MIME.Value, nil
	} else if strings.HasPrefix(kind.MIME.Value, "video") && fileType == "video" {
		width, height, bhash, nil := GetVideoDimensions(filePath)
		return width, height, bhash, kind.MIME.Value, nil
	} else {
		return 0, 0, "", "", errors.New("unsupported media type")
	}
}

func ExtractMediaInfo(imagePath string, fileType string) (int, int, int64, string, string, string, error) {
	width, height, bhash, mime, err := GetMediaDimensions(imagePath, fileType) // Assuming the same function can be used for images
	if err != nil {
		return 0, 0, 0, "", "", "", fmt.Errorf("GetMediaDimensions: %v", err)
	}

	file, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, 0, "", "", "", fmt.Errorf("open(%s): %v", imagePath, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return 0, 0, 0, "", "", "", fmt.Errorf("hashing %s: %v", imagePath, err)
	}
	fileHash := fmt.Sprintf("%x", hash.Sum(nil))

	// get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, 0, 0, "", "", "", fmt.Errorf("stat(%s): %v", imagePath, err)
	}
	fileSize := fileInfo.Size()

	return width, height, fileSize, fileHash, bhash, mime, nil
}

// LoadImage loads an image from the specified file path.
func LoadImage(filePath string) (image.Image, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open(%s): %v", filePath, err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode(%s): %v", filePath, err)
	}

	return img, nil
}

// ExtractFrameFromVideo extracts a frame from the video file and saves it as an image.
func ExtractFrameFromVideo(videoPath string) (string, error) {
	framePath := filepath.Join(os.TempDir(), "frame.jpg")
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-ss", "00:00:01.000", "-vframes", "1", framePath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("extracting frame from video: %v", err)
	}
	return framePath, nil
}

func UploadFile(server, filePath string, signer nostr.Keyer) (map[string]interface{}, error) {
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
		return nil, fmt.Errorf("upload failed: %s, code %d", string(bodyBytes), resp.StatusCode)
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
func createAuthorizationEvent(signer nostr.Keyer, verb string, tags [][]string) (string, error) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pubKeyHex, err := signer.GetPublicKey(ctx)
	if err != nil {
		return "", err
	}
	event.PubKey = pubKeyHex

	// Sign the event
	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()
	err = signer.SignEvent(ctx2, &event)
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

func PublishEvent(event *nostr.Event, signer nostr.Keyer, relays []string) {
	for _, relayURL := range relays {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		relay, err := nostr.RelayConnect(ctx, relayURL)
		if err != nil {
			log.Printf("Error connecting to relay %s: %v", relayURL, err)
			continue
		}
		defer relay.Close()

		err = relay.Publish(ctx, *event)
		if err != nil {
			// longer timeout because it might involve a remote signature
			ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel2()

			if strings.HasPrefix(err.Error(), "msg: auth-required:") {

				authErr := relay.Auth(ctx2, func(authEvent *nostr.Event) error {
					return signer.SignEvent(ctx2, authEvent)
				})
				if authErr != nil {
					log.Printf("Error sending auth event to relay %s: %v", relayURL, authErr)
					continue
				}

				err = relay.Publish(ctx2, *event)
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

func Pow(event *nostr.Event, diff int) error {
	if diff > 0 {
		nounce, err := nip13.DoWork(context.Background(), *event, diff)
		if err != nil {
			return fmt.Errorf("error generating proof of work: %v", err)
		}
		event.Tags = append(event.Tags, nounce)
	}
	return nil
}

// ExtractHashtags extracts hashtags from a given text
func ExtractHashtags(event *nostr.Event) {
	if event.Content == "" {
		return
	}
	re := regexp.MustCompile(`#\w+`)
	tags := re.FindAllString(event.Content, -1)
	for _, tag := range tags {
		event.Tags = append(event.Tags, nostr.Tag{"t", strings.TrimPrefix(tag, "#")})
	}
}
