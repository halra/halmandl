# Go File Downloader

A Go package for downloading files with support for concurrent downloads and resumable downloads.

## Features

- Concurrent file downloads in parts
- Resumable downloads with support for interruption recovery
- Configurable number of concurrent parts and junk size
- Progress tracking with download speed and completion percentage
- Retry mechanism for robust downloads
- Uses Go's built-in packages and minimal dependencies

## Installation

To use this package in your Go project, you can simply import it:

```go
import "github.com/halra/halmandl"
```

Then, create a downloader instance and start downloading files.

```go
downloader := halmandl.NewDownloader()
err := downloader.Download("download_directory", "file_url")
if err != nil {
    log.Fatal(err)
}
```

## Usage

```go
import (
    "github.com/halra/halmandl"
    "log"
)

func main() {
    // Create a downloader instance with custom options
    downloader := halmandl.NewDownloader()
    downloader.Options.ConcurrentParts = 4
    downloader.Options.JunkSize = 1024 * 1024 // 1 MB
    downloader.Options.UseStats = true

    // Set a custom logger (optional)
    customLogger := log.New(os.Stdout, "Downloader: ", log.LstdFlags)
    downloader.SetLogger(customLogger)

    // Start downloading a file
    err := downloader.Download("download_directory", "file_url")
    if err != nil {
        log.Fatal(err)
    }
}
```

## Configuration Options

- `ConcurrentParts`: The number of parts to download concurrently (default: 1)
- `JunkSize`: The size of each download part (default: 4 MB)
- `UseStats`: Enable progress tracking statistics (default: false)
- `MaxTries`: Maximum number of download retries (default: 10)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! If you have any suggestions, improvements, or bug fixes, please open an issue or create a pull request.

## Acknowledgments

- This project was inspired by the need for a robust file downloader in Go.
- Special thanks to the Go community and contributors.

