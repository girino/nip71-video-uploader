package utils

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
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
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0", filePath)
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

// EncodeEventData encodes the event data for NIP 71 compliance.
func EncodeEventData(eventData interface{}) (string, error) {
	// Implementation for encoding event data goes here.
	// This is a placeholder for actual encoding logic.
	return "", nil
}
