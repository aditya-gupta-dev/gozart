# Gozart

> **Automated YouTube Shorts Pipeline** — Download, loop, and upload music videos at scale with concurrent processing.

Gozart is a high-performance CLI tool for automating YouTube Shorts creation. It downloads videos, extracts audio, merges with custom visual assets, loops content to target durations, and uploads the final product back to YouTube with thumbnails and metadata.

---

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Process Command](#process-command)
  - [Randomize Command](#randomize-command)
  - [Clean Command](#clean-command)
- [Project Structure](#project-structure)
- [Workflow](#workflow)
- [API Reference](#api-reference)
- [Advanced Configuration](#advanced-configuration)
- [Troubleshooting](#troubleshooting)
- [License](#license)

---

## Features

- **Concurrent Processing**: Process multiple videos simultaneously with configurable concurrency limits
- **Automated Video Pipeline**: Download → Extract Audio → Merge with Asset → Loop → Upload
- **YouTube API Integration**: Automatic upload with metadata, tags, and thumbnail attachment
- **Smart Caching**: Skip already-processed videos and reuse intermediate outputs
- **Flexible Modes**: Process specific videos or randomly select from downloaded content
- **Progress Tracking**: Real-time upload progress bars and comprehensive logging
- **Configurable Duration**: Automatically loop videos to target length (e.g., 30s, 1hr, 3hr)
- **OAuth2 Authentication**: Secure YouTube API access with token persistence

---

## Architecture

```
┌─────────────┐
│   CLI       │
│  (Cobra)    │
└──────┬──────┘
       │
       ├───► process    (Download → Edit → Upload)
       ├───► randomize  (Random selection processor)
       └───► clean      (Cleanup artifacts)
              │
    ┌─────────┴─────────┐
    │                   │
┌───▼─────┐      ┌──────▼──────┐
│Downloader│      │   Editor    │
│ (yt-dlp) │      │  (ffmpeg)   │
└──────────┘      └──────┬──────┘
                         │
                  ┌──────▼──────┐
                  │  Uploader   │
                  │ (YouTube v3)│
                  └─────────────┘
```

### Package Overview

| Package | Purpose |
|---------|---------|
| **cmd** | CLI commands (process, randomize, clean) |
| **config** | Configuration loading and validation |
| **downloader** | YouTube video download via yt-dlp |
| **editor** | Video processing, audio extraction, looping |
| **uploader** | YouTube API upload with OAuth2 |
| **logger** | Timestamped file and stdout logging |

---

## Prerequisites

### Required Tools

- **Go** 1.26.1+ (for building from source)
- **ffmpeg** — Video/audio processing
- **ffprobe** — Media duration calculation
- **yt-dlp** — YouTube video downloader

### Installation Commands

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install ffmpeg yt-dlp

# macOS (Homebrew)
brew install ffmpeg yt-dlp

# Windows (Chocolatey)
choco install ffmpeg yt-dlp
```

### YouTube API Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing
3. Enable **YouTube Data API v3**
4. Create OAuth 2.0 credentials (Desktop App)
5. Download `client_secrets.json` to project root
6. Create API key for video metadata fetching

---

## Installation

### Option 1: Build from Source

```bash
git clone <repository-url>
cd gozart
go build -o gozart
chmod +x gozart
```

### Option 2: Direct Execution

```bash
go run main.go [command] [flags]
```

---

## Configuration

Create a `config.json` file in your working directory:

```json
{
  "ffmpegPath": "ffmpeg",
  "ytdlpPath": "yt-dlp",
  "final-video-duration": "30",
  "youtube-api-key": "YOUR_YOUTUBE_API_KEY_HERE",
  "asset-video-path": "sample.mp4",
  "output-directory": "output",
  "links-file-path": "links.txt"
}
```

### Configuration Parameters

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `ffmpegPath` | string | Path to ffmpeg binary | `ffmpeg` |
| `ytdlpPath` | string | Path to yt-dlp binary | `yt-dlp` |
| `final-video-duration` | string | Target video length in seconds | `30` |
| `youtube-api-key` | string | YouTube Data API v3 key | **Required** |
| `asset-video-path` | string | Background video to merge with audio | `sample.mp4` |
| `output-directory` | string | Final render output location | `output` |
| `links-file-path` | string | File containing YouTube URLs (one per line) | `links.txt` |

### Additional Required Files

- **`client_secrets.json`** — OAuth2 credentials from Google Cloud Console
- **`links.txt`** — YouTube video URLs (one per line)
- **`sample.mp4`** — Asset video for visual background

---

## Usage

### Process Command

Download, edit, and optionally upload videos from `links.txt`.

```bash
./gozart process [flags]
```

#### Flags

| Flag | Shorthand | Type | Default | Description |
|------|-----------|------|---------|-------------|
| `--upload` | `-u` | bool | `false` | Upload to YouTube after processing |
| `--concurrency` | `-c` | int | `2` | Number of videos to process simultaneously |

#### Examples

```bash
# Process videos without uploading (dry run)
./gozart process

# Process and upload with default concurrency (2)
./gozart process --upload

# High-throughput processing with 5 concurrent workers
./gozart process -u -c 5

# Single-threaded processing
./gozart process --upload --concurrency 1
```

---

### Randomize Command

Randomly select and process videos from already-downloaded content in `files/` directory.

```bash
./gozart randomize [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--mode` | string | `one` | Selection mode: `one`, `few`, or `all` |

#### Modes

| Mode | Behavior |
|------|----------|
| `one` | Process a single random video |
| `few` | Process random subset (1 to N videos) |
| `all` | Process all downloaded videos |

#### Examples

```bash
# Process one random video
./gozart randomize

# Process random subset
./gozart randomize --mode few

# Process all downloaded videos
./gozart randomize --mode all
```

**Note**: Randomize always uploads to YouTube (hardcoded behavior).

---

### Clean Command

Remove generated files and artifacts.

```bash
./gozart clean [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--reset` | bool | `false` | Also delete asset video and clear links.txt |

#### Cleanup Targets

| Target | Description |
|--------|-------------|
| `output/` | Final rendered videos |
| `files/` | Downloaded videos and intermediate files |
| `*.log` | All log files in current directory |
| `sample.mp4` (with --reset) | Asset video |
| `links.txt` (with --reset) | Emptied but not deleted |

#### Examples

```bash
# Remove generated files only
./gozart clean

# Full reset (removes asset and clears links)
./gozart clean --reset
```

---

## Project Structure

```
gozart/
├── main.go                 # Application entry point
├── go.mod                  # Go module dependencies
├── go.sum                  # Dependency checksums
├── config.json            # Configuration file (create this)
├── client_secrets.json    # OAuth2 credentials (create this)
├── links.txt              # YouTube URLs to process (create this)
├── sample.mp4             # Asset video for background (provide this)
├── gozart                 # Compiled binary
│
├── cmd/                   # CLI Commands
│   ├── root.go           # Root command definition
│   ├── process.go        # Process command (main pipeline)
│   ├── randomize.go      # Randomize command
│   └── clean.go          # Clean command
│
└── pkg/                   # Core Packages
    ├── config/
    │   └── config.go     # Configuration loader and validator
    ├── logger/
    │   └── logger.go     # Dual output logger (file + stdout)
    ├── downloader/
    │   └── downloader.go # yt-dlp wrapper for video downloads
    ├── editor/
    │   └── editor.go     # FFmpeg video processing pipeline
    └── uploader/
        └── uploader.go   # YouTube API v3 upload client

Generated during execution:
├── files/                 # Downloaded videos (organized by video ID)
│   └── {videoID}/
│       ├── input.webm    # Original download
│       ├── input.mp4     # Converted input (if needed)
│       ├── audio.mp3     # Extracted audio
│       ├── final_output.mp4  # Asset + audio merged
│       └── files.txt     # FFmpeg concat demuxer file
│
├── output/                # Final rendered videos
│   └── {videoID}/
│       ├── {title}_X_Hour_looped.mp4  # Final render
│       └── {videoID}.jpg              # Thumbnail
│
├── token.json             # OAuth2 token (auto-generated)
└── {timestamp}.log        # Execution logs
```

---

## Workflow

### Full Processing Pipeline (`process` command)

```
1. Initialization
   ├─ Load config.json
   ├─ Validate ffmpeg, ffprobe, yt-dlp
   └─ Authenticate YouTube API (if --upload)

2. Download Phase (Parallel)
   ├─ Read links.txt
   ├─ Extract video IDs
   ├─ Download to files/{videoID}/input.{ext}
   └─ Skip if already downloaded

3. Edit Phase (Per Video)
   ├─ Convert to MP4 (if downloaded as .webm)
   ├─ Extract audio to audio.mp3
   ├─ Merge asset video + audio → final_output.mp4
   ├─ Calculate duration using ffprobe
   ├─ Generate concat demuxer file (files.txt)
   ├─ Loop video to target duration
   └─ Download YouTube thumbnail

4. Upload Phase (if --upload)
   ├─ Fetch video title via YouTube API
   ├─ Generate sanitized filename
   ├─ Upload video with metadata
   ├─ Set privacy to "private"
   └─ Upload thumbnail
```

### File Transformations

```
input.webm
    ↓ (ConvertToMp4)
output.mp4 ─────────────┐
                        ├─ (ExtractAudio) → audio.mp3
asset video (sample.mp4)┘
    ↓
final_output.mp4 (merged)
    ↓ (GenerateConcatDemuxerFile)
files.txt (repetition list)
    ↓ (RenderFinalOutputVideo)
{title}_X_Hour_looped.mp4 (final render)
```

---

## API Reference

### Editor Package

```go
type VideoEditor struct {
    Link                     string
    Logger                   *logger.Logger
    ConfigLoader             *config.ConfigLoader
    Failed                   bool
    AlreadyAsMp4             bool
    VideoID                  string
    FinalOutputVideoDuration int
}
```

#### Key Methods

| Method | Description |
|--------|-------------|
| `Edit()` | Execute full editing pipeline |
| `GetVideoTitle()` | Fetch title from YouTube API |
| `GenerateOutputFilename(title string)` | Sanitize and format output filename |
| `ConvertToMp4()` | Convert .webm to .mp4 |
| `ExtractAudioFromVideo()` | Extract audio track as .mp3 |
| `MergeAssetAndAudioFile()` | Merge asset video with extracted audio |
| `GetVideoDuration()` | Calculate video duration via ffprobe |
| `GenerateConcatDemuxerFile()` | Create FFmpeg concat file for looping |
| `RenderFinalOutputVideo()` | Concatenate loops and render final video |
| `DownloadOriginalThumbnail()` | Download YouTube thumbnail |

### Uploader Package

```go
type YouTubeUploader struct {
    logger           *logger.Logger
    configLoader     *config.ConfigLoader
    clientSecretFile string
    tokenFile        string
    Service          *youtube.Service
}
```

#### Key Methods

| Method | Description |
|--------|-------------|
| `Authenticate()` | OAuth2 authentication flow |
| `UploadVideo(videoFile, title, description, tags)` | Upload video with metadata |
| `UploadThumbnail(videoID, thumbnailPath)` | Set custom thumbnail |

### Downloader Package

```go
type VideoDownloader struct {
    logger       *logger.Logger
    configLoader *config.ConfigLoader
    tempDir      string
}
```

#### Key Methods

| Method | Description |
|--------|-------------|
| `GetVideoID(url)` | Extract video ID from YouTube URL |
| `GetLinksFromFile()` | Read and parse links.txt |
| `DownloadVideoUsingPkg(link)` | Download video via yt-dlp |

---

## Advanced Configuration

### Custom FFmpeg/yt-dlp Paths

If binaries are not in PATH:

```json
{
  "ffmpegPath": "/usr/local/bin/ffmpeg",
  "ytdlpPath": "/opt/homebrew/bin/yt-dlp"
}
```

### Long-Form Looped Videos

For 1-hour loops:

```json
{
  "final-video-duration": "3600"
}
```

**Formula**: `repetitions = ceil(targetDuration / videoDuration)`

### High-Resolution Asset Video

Replace `sample.mp4` with high-quality background:

```bash
# 1080x1920 (9:16 aspect ratio for Shorts)
ffmpeg -i input.mp4 -vf "scale=1080:1920:force_original_aspect_ratio=decrease,pad=1080:1920:(ow-iw)/2:(oh-ih)/2" -c:a copy sample.mp4
```

---

## Troubleshooting

### Common Issues

#### 1. "Not found config.json"

**Solution**: Create `config.json` in the directory where you run the CLI.

```bash
pwd  # Verify working directory
ls config.json  # Check file exists
```

#### 2. "Asset video file not found"

**Solution**: Ensure `sample.mp4` exists or update path in config.json.

```bash
ls -l sample.mp4
```

#### 3. "ffmpeg check failed"

**Solution**: Install ffmpeg or set correct path.

```bash
which ffmpeg  # Find ffmpeg location
ffmpeg -version  # Verify installation
```

#### 4. OAuth2 Authentication Loop

**Solution**: Delete `token.json` and re-authenticate.

```bash
rm token.json
./gozart process --upload  # Will prompt for new auth
```

#### 5. "Failed to download the video"

**Possible Causes**:
- Private/age-restricted video
- Geographic restrictions
- Invalid URL format

**Solution**: Check video accessibility and URL format.

```bash
yt-dlp --simulate <URL>  # Test download manually
```

### Logs

Check timestamped log files in current directory:

```bash
ls -lt *.log | head -1  # Latest log
tail -f <latest>.log    # Follow log in real-time
```

---

## Performance Tips

### Optimal Concurrency

```bash
# For CPU-bound systems (video encoding)
./gozart process -u -c $(nproc)  # Use all CPU cores

# For I/O-bound systems (downloads)
./gozart process -u -c 10
```

### Disk Space Management

```bash
# Remove intermediate files after successful upload
./gozart clean

# Monitor disk usage
du -sh files/ output/
```

### Rate Limiting

YouTube API has quota limits:
- **10,000 units/day** (free tier)
- Video upload = **1,600 units**
- Thumbnail upload = **50 units**

**Daily capacity**: ~6 videos with thumbnails

---

## Dependencies

### Core Libraries

| Library | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/schollz/progressbar/v3` | Upload progress visualization |
| `golang.org/x/oauth2` | OAuth2 authentication |
| `google.golang.org/api` | YouTube Data API v3 client |

### Full Dependency Tree

See `go.mod` for complete list.

---

## License

This project is provided as-is for educational and automation purposes. Ensure compliance with YouTube's Terms of Service when using this tool.

---

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## Support

For issues and questions:

1. Check [Troubleshooting](#troubleshooting) section
2. Review logs in `{timestamp}.log` files
3. Open an issue with log excerpts and `config.json` (redact API keys)

---

**Built with Go • Powered by FFmpeg & yt-dlp • Automated by Gozart**
