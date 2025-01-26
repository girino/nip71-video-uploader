# Go CLI Utility for NIP 68 and NIP 71 Events

This project is a command line utility written in Go that creates events compatible with NIP 68 for images and NIP 71 for high-quality video files. It takes a filename and a private key as command line arguments and generates the necessary event structure.

## Project Structure

```
go-cli-utility
├── cmd
│   ├── nip68
│   │   └── main.go      # Entry point for NIP 68 image events
│   └── nip71
│       └── main.go      # Entry point for NIP 71 video events
├── internal
│   └── utils
│       └── utils.go     # Utility functions for processing media files
├── relays.json          # JSON file containing the list of relays
├── go.mod               # Module definition and dependencies
└── README.md            # Project documentation
```

## Installation

To install the project, clone the repository and navigate to the project directory:

```bash
git clone <repository-url>
cd go-cli-utility
```

Then, run the following command to download the necessary dependencies:

```bash
go mod tidy
```

## Usage

### NIP 68 Image Events

To run the command line utility for NIP 68 image events, use the following syntax:

```bash
go run cmd/nip68/main.go -file <image_file> -url <image_url> -key <private_key> [-title <title>] [-description <description>] [-published_at <timestamp>] [-relay <relay_address_or_file>]
```

#### Parameters

- `-file`: Path to the image file (required, can be specified multiple times)
- `-url`: URL of the image file (required, can be specified multiple times)
- `-key`: Private key for signing the event (required)
- `-title`: Title of the image (optional, defaults to the image filename)
- `-description`: Description of the image (optional, defaults to an empty string)
- `-published_at`: Timestamp when the image was published in Unix seconds (optional, defaults to the current time)
- `-relay`: Relay address or path to relays.json file (optional)

#### Example

```bash
go run cmd/nip68/main.go -file path/to/image1.jpg -file path/to/image2.jpg -url https://example.com/image3.jpg -key my_private_key -title "My Image" -relay relays.json
```

### NIP 71 Video Events

To run the command line utility for NIP 71 video events, use the following syntax:

```bash
go run cmd/nip71/main.go -url <video_url> -key <private_key> [-title <title>] [-description <description>] [-published_at <timestamp>] [-relay <relay_address_or_file>]
```

#### Parameters

- `-url`: URL of the video file (required)
- `-file`: Path to the video file (required if `-url` is not provided)
- `-key`: Private key for signing the event (required)
- `-title`: Title of the video (optional, defaults to the video URL)
- `-description`: Description of the video (optional, defaults to an empty string)
- `-published_at`: Timestamp when the video was published in Unix seconds (optional, defaults to the current time)
- `-relay`: Relay address or path to relays.json file (optional)

#### Example

```bash
go run cmd/nip71/main.go -url https://example.com/video.mp4 -key my_private_key -title "My Video" -relay relays.json
```

### Configuring Relays

The list of relays can be provided as a JSON file or directly as a relay address. If the `-relay` parameter starts with `ws://` or `wss://`, it will be considered as a relay to add to the list. If it is an existing file, it will be loaded. If not present, the events will not be relayed.

Example `relays.json` file:

```json
[
	"wss://relay.primal.net",
	"wss://wot.girino.org",
	"wss://nostr.girino.org",
	"wss://haven.girino.org/outbox",
	"wss://haven.girino.org/private"
]
```

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue for any suggestions or improvements.

## License

This project is licensed under the MIT License. See the LICENSE file for more details.