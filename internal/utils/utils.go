package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/nbd-wtf/go-nostr/nip19"
)

// ValidateInput checks if the provided video URL, private key, title, and published_at are valid
func ValidateInput(videoURL, privateKey, title, publishedAt string) error {
	if videoURL == "" {
		return errors.New("video URL cannot be empty")
	}

	if _, err := url.ParseRequestURI(videoURL); err != nil {
		return errors.New("invalid video URL")
	}

	if privateKey == "" {
		return errors.New("private key cannot be empty")
	}

	if title == "" {
		return errors.New("title cannot be empty")
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

	if resp.StatusCode != http.StatusOK {
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

// GetVideoDimensions uses ffprobe to get the dimensions of the video
func GetVideoDimensions(filePath string) (int, int, error) {

	// Validate if the file is a video using filetype library
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

	if kind == types.Unknown || !strings.HasPrefix(kind.MIME.Value, "video") {
		return 0, 0, errors.New("the file is not a valid video")
	}

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

func UploadFile(server, filePath, privKey string) (map[string]interface{}, error) {
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
	authEventJSON, err := createAuthorizationEvent(privKey, "upload", [][]string{
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
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
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
func createAuthorizationEvent(privKey string, verb string, tags [][]string) (string, error) {
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

	// Convert the private key from hex
	secKey := privKey
	if strings.HasPrefix(privKey, "nsec") {
		var err error
		secKey, err = convertNIP19ToHex(privKey)
		if err != nil {
			return "", err
		}
	}

	// Set the pubkey
	pubKeyHex, err := nostr.GetPublicKey(secKey)
	if err != nil {
		return "", err
	}
	event.PubKey = pubKeyHex

	// Sign the event
	err = event.Sign(secKey)
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

func convertNIP19ToHex(key string) (string, error) {
	if strings.HasPrefix(key, "nsec") {
		_, decoded, err := nip19.Decode(key)
		if err != nil {
			return "", fmt.Errorf("failed to decode NIP-19 key: %v", err)
		}
		return decoded.(string), nil
	}
	return key, nil
}
