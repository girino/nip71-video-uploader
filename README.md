# Go CLI Utility for NIP 71 Video Events

This project is a command line utility written in Go that creates events compatible with NIP 71 for high-quality video files. It takes a filename and a private key as command line arguments and generates the necessary event structure.

## Project Structure

```
go-cli-utility
├── cmd
│   └── main.go          # Entry point of the command line utility
├── internal
│   ├── event
│   │   └── event.go     # Defines the event structure and methods
│   └── utils
│       └── utils.go     # Utility functions for processing video files
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

To run the command line utility, use the following syntax:

```bash
go run cmd/main.go -url <video_url> -key <private_key> [-title <title>] [-description <description>] [-published_at <timestamp>] [-duration <duration>]
```
### Parameters

-url: URL of the video file (required)
-key: Private key for signing the event (required)
-title: Title of the video (optional, defaults to the video URL)
-description: Description of the video (optional, defaults to an empty string)
-published_at: Timestamp when the video was published in Unix seconds (optional, defaults to the current time)
-duration: Duration of the video in seconds (optional)

### Example

```bash
go run cmd/main.go -url https://example.com/video.mp4 -key my_private_key -title "My Video" -description "This is a description" -published_at 1633024800 -duration 120
```

This command will create an event for the video file `video.mp4` using the provided private key.

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue for any suggestions or improvements.

## License

This project is licensed under the MIT License. See the LICENSE file for more details.