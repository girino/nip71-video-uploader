package event

import (
	"encoding/json"
	"errors"
	"os"
	"time"
)

type Event struct {
	Kind       int       `json:"kind"`
	Tags       []string  `json:"tags"`
	Content    string    `json:"content"`
	PubKey     string    `json:"pubkey"`
	Sig        string    `json:"sig"`
	CreatedAt  time.Time `json:"created_at"`
}

func NewEvent(filename string, privateKey string) (*Event, error) {
	if filename == "" || privateKey == "" {
		return nil, errors.New("filename and private key must not be empty")
	}

	// Here you would add logic to extract video metadata and create the event content
	content := "Video file: " + filename // Placeholder for actual video content
	tags := []string{"video", filename}

	event := &Event{
		Kind:      1, // Assuming 1 is the kind for video events
		Tags:      tags,
		Content:   content,
		PubKey:    privateKey, // This should be the public key derived from the private key
		CreatedAt: time.Now(),
	}

	return event, nil
}

func (e *Event) ToJSON() (string, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (e *Event) SaveToFile(filepath string) error {
	jsonData, err := e.ToJSON()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, []byte(jsonData), 0644)
}