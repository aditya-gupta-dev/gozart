# Gozart: Autonomous AI YouTube Video Generation Engine
## Architectural Blueprint & Technical Implementation Plan (`plan.md`)

---

## 1. Executive Summary & Vision

**Gozart** is evolving from a single-purpose video looper CLI into an **end-to-end autonomous YouTube creation and distribution platform**. Given only a **Video Idea** and a **Target Country/Audience**, Gozart conducts competitive research, identifies breakout viral videos in that region, mines real audience comments for sentiment and curiosity gaps, synthesizes a high-retention script, generates voiceover and programmatic visuals in parallel, polishes audio with deep-learning noise suppression, creates high-CTR thumbnails, optimizes YouTube SEO, and offers a self-hosted **Go Templates + HTMX web dashboard** for monitoring, previewing, and one-click publishing.

---

## 2. Decision Matrix: Best Technology Selections & Rationale

Every decision has been vetted against three criteria: **performance**, **operational simplicity (zero unnecessary infrastructure)**, and **output quality**.

| Pipeline Stage | Evaluated Options | Chosen Technology | Why It Is the Best Choice |
| :--- | :--- | :--- | :--- |
| **Orchestrator & Core Engine** | Go vs. Python vs. Node.js | **Go (1.26+)** | High-concurrency goroutines, native binary deployment, rock-solid subprocess management (FFmpeg/yt-dlp), fast startup, low memory footprint. |
| **Web Frontend & UI** | React/Next.js vs. Vue vs. Go `html/template` + HTMX | **Go `html/template` + HTMX + Tailwind CSS** | Embedded directly into Go binary (`embed.FS`). Zero Node.js runtime required for the web server, instant SSR, real-time log and progress streaming via Server-Sent Events (SSE). |
| **Agent & LLM Harness** | LangChain / CrewAI vs. Native Go Ollama API | **Native Go Ollama API (`github.com/ollama/ollama/api`) + OpenAI-compatible Client** | Direct Go integration without heavy Python agent framework bloat. Full support for local models (`qwen2.5:32b`, `llama3.3:70b`) and cloud fallback (`gemini-1.5-pro`, `gpt-4o`). |
| **Research & YouTube Tool** | Raw Web Scraping vs. YouTube Data API v3 + yt-dlp | **YouTube Data API v3 + yt-dlp fallback** | Official API delivers accurate metadata, region-specific search (`regionCode`), view velocity, and comment threads. `yt-dlp` provides resilient transcript extraction when captions are disabled via API. |
| **Comment Sentiment Analysis** | Classical NLP (VADER/NLTK) vs. LLM Structured Extraction | **LLM Zero-Shot Structured Extraction (`qwen2.5:14b` / `gpt-4o-mini`)** | Captures sarcasm, niche humor, emotional objections, and domain-specific questions that classical rule-based NLP completely misses. |
| **Voiceover (TTS)** | ElevenLabs API vs. Edge-TTS vs. Kokoro-82M | **Kokoro-82M (Local) + Edge-TTS (Zero-GPU Fallback)** | Kokoro-82M produces studio-grade, natural human voiceover locally with ultra-fast inference and zero API cost. Edge-TTS provides a zero-setup fallback. |
| **Voice Polish / Noise Filter** | RNNoise vs. Audacity Macro vs. DeepFilterNet | **DeepFilterNet v3 (Rust / PyTorch)** | State-of-the-art deep learning speech enhancement by Facebook/FAIR. Operates at 48kHz, removes room reverb, mic hiss, and compression artifacts without robotic voice degradation. |
| **Media Code Generation** | MoviePy vs. Manim vs. Remotion | **Remotion (React/TypeScript) & Manim CE (Python)** | *Remotion* is the gold standard for high-retention 9:16 Shorts (kinetic typography, split screens, progress bars, audio sync). *Manim* is optimal for 16:9 scientific/explainer visuals. |
| **Thumbnail Generation** | DALL-E 3 vs. Stable Diffusion 1.5 vs. FLUX.1-schnell / SDXL LoRA | **FLUX.1-schnell (via local ComfyUI/Diffusers API) or SDXL YouTube LoRA** | FLUX delivers photorealistic textures, dynamic lighting, and crisp composition required for high YouTube click-through rates (CTR). |
| **Compositing & Mastering** | MoviePy vs. GStreamer vs. FFmpeg | **FFmpeg (via Go exec pipe)** | Native C performance, zero re-encoding where stream-copy is possible, automated audio ducking via `sidechaincompress`, and standard `-14 LUFS` EBU R128 loudness normalization. |
| **State & Checkpointing** | Redis / Postgres vs. SQLite (WAL) | **Embedded SQLite (`modernc.org/sqlite`)** | Single file database, zero external daemon setup, cross-platform, ACID compliant with WAL mode for concurrent multi-job progress tracking. |

---

## 3. Detailed End-to-End System Architecture

```
                  ┌────────────────────────────────────────────────────────┐
                  │                 Gozart Hosted Web UI                   │
                  │        (Go html/template + HTMX + Tailwind)            │
                  └───────────┬────────────────────────────────┬───────────┘
                              │ Trigger Job                    │ SSE Live Updates
                              ▼                                ▼
                  ┌────────────────────────────────────────────────────────┐
                  │               Go Pipeline Orchestrator                 │
                  │   - Job Scheduler      - State Machine Checkpointing   │
                  │   - Concurrency Worker - SQLite Job & Artifact DB      │
                  └───────────────────────────┬────────────────────────────┘
                                              │
                      ┌───────────────────────┴───────────────────────┐
                      ▼                                               ▼
     ┌─────────────────────────────────┐             ┌─────────────────────────────────┐
     │      AI Research Agent          │             │    Comment Sentiment Engine     │
     │ Tool: [search-youtube-videos]   │             │ - Scrapes Top 100 Comments      │
     │ - YouTube API (Region/Country)  │────────────►│ - Clusters Viewer Emotion       │
     │ - yt-dlp Subtitle Extractor     │             │ - Extracts Unanswered Questions │
     │ - Velocity & Outlier Ranking    │             │ - Identifies Controversy Hooks  │
     └─────────────────────────────────┘             └────────────────┬────────────────┘
                                                                      │
                                                                      ▼
                                                     ┌─────────────────────────────────┐
                                                     │        Scriptwriting LLM        │
                                                     │ Synthesizes Transcripts +       │
                                                     │ Viral Metrics + Audience Pain   │
                                                     │ Output: Strict Scene JSON Spec  │
                                                     └────────────────┬────────────────┘
                                                                      │
                 ┌───────────────────────┬────────────────────────────┼──────────────────────────┐
                 ▼                       ▼                            ▼                          ▼
     ┌───────────────────────┐ ┌───────────────────────┐  ┌───────────────────────┐  ┌───────────────────────┐
     │  Voiceover Pipeline   │ │  Media Code Pipeline  │  │  Thumbnail Generation │  │   YouTube SEO Model   │
     │ 1. TTS Generation     │ │ 1. Code Gen LLM       │  │ 1. Prompt Crafter LLM │  │ 1. High-CTR Title Gen │
     │    (Kokoro / EdgeTTS) │ │    (Remotion/Manim)   │  │ 2. Fine-Tuned FLUX /  │  │ 2. Search-Optimized   │
     │ 2. DeepFilterNet v3   │ │ 2. Headless Render    │  │    SDXL Diffusion     │  │    Description        │
     │    (48kHz Enhancement)│ │ 3. Self-Healing Loop  │  │ 3. Contrast & Sharpness│  │ 3. Competitive Tags   │
     └───────────┬───────────┘ └───────────┬───────────┘  └───────────┬───────────┘  └───────────┬───────────┘
                 │                         │                          │                          │
                 └───────────┬─────────────┘                          │                          │
                             ▼                                        │                          │
     ┌─────────────────────────────────────────────────┐              │                          │
     │               FFmpeg Compositor                 │              │                          │
     │ - Multiplex Media Video + Voiceover Audio       │              │                          │
     │ - Sidechain Audio Ducking (-14dB for BGM)       │              │                          │
     │ - Loudness Normalization (-14 LUFS)             │              │                          │
     │ - Burn-in Dynamic Animated Subtitles (.ass)     │              │                          │
     └───────────────────────┬─────────────────────────┘              │                          │
                             │                                        │                          │
                             ▼                                        ▼                          ▼
     ┌───────────────────────────────────────────────────────────────────────────────────────────┐
     │                           YouTube Uploader (OAuth2 API v3)                                │
     │ Uploads MP4 Video + Applies Thumbnail + Sets SEO Metadata + Privacy (Public/Draft/Private)│
     └───────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Minute Technical Breakdown: Every Step & Subsystem

### Step 1: Input & Configuration Subsystem

#### CLI & Web Inputs:
* `topic`: Topic or premise (e.g., *"Why NASA stopped sending humans to the moon"*).
* `target_country`: ISO 3166-1 alpha-2 code (e.g., `US`, `GB`, `IN`, `CA`, `AU`).
* `target_format`: `shorts_9_16` (1080x1920) or `longform_16_9` (1920x1080).
* `target_duration_sec`: Target length (e.g., 55s for Shorts, 480s for Longform).
* `tone`: `sensational`, `educational`, `storytelling`, `humorous`, `mystery`.
* `model_tier`: `local` (Ollama) or `cloud` (OpenAI / Gemini / Anthropic).

#### Configuration File (`config.json`):
```json
{
  "server": {
    "port": 8080,
    "host": "0.0.0.0"
  },
  "ai": {
    "provider": "ollama",
    "ollama_base_url": "http://localhost:11434",
    "agent_model": "qwen2.5:32b",
    "fast_model": "qwen2.5:7b",
    "api_key": ""
  },
  "youtube": {
    "api_key": "YOUR_API_KEY",
    "client_secrets_path": "client_secrets.json",
    "token_cache_path": "token.json"
  },
  "audio": {
    "tts_engine": "kokoro",
    "voice": "af_bella",
    "deepfilter_enabled": true,
    "deepfilter_path": "deep-filter"
  },
  "media": {
    "engine": "remotion",
    "node_binary": "node",
    "max_healing_retries": 3
  },
  "storage": {
    "db_path": "gozart.db",
    "workdir": "workspace",
    "output_dir": "output"
  }
}
```

---

### Step 2: AI Research Agent & `[search-youtube-videos]` Tool

#### 1. Tool Declaration & Schema (Ollama / OpenAI Format)
The Agent is equipped with a native Go tool definition:
```json
{
  "type": "function",
  "function": {
    "name": "search_youtube_videos",
    "description": "Searches for top performing viral YouTube videos for a given query, language, and country code, returning metrics and video IDs.",
    "parameters": {
      "type": "object",
      "properties": {
        "query": { "type": "string", "description": "Target search query or keywords" },
        "country_code": { "type": "string", "description": "2-letter ISO country code (e.g., US, IN, GB)" },
        "max_results": { "type": "integer", "description": "Number of videos to evaluate (5-25)" }
      },
      "required": ["query", "country_code"]
    }
  }
}
```

#### 2. Execution Logic (`pkg/research/research.go`):
1. **Query Construction**: The Agent formulates 3 distinct search variations based on the user's topic and target country (e.g. `[topic] explained`, `[topic] truth`, `why [topic]`).
2. **YouTube API Call**:
   * Uses `youtube.NewService()` with `service.Search.List("id,snippet")`.
   * Sets `RegionCode(countryCode)`, `RelevanceLanguage("en")`, `Type("video")`, `Order("relevance")`.
3. **Metrics Enrichment (`videos.list`)**:
   * Fetches `statistics` (viewCount, likeCount, commentCount) and `contentDetails` (duration).
   * **Calculates Outlier Velocity Score**:
     $$Velocity = \frac{\text{ViewCount}}{\text{Hours Since Published} + 2}$$
     $$\text{Engagement Rate} = \frac{\text{LikeCount} + \text{CommentCount}}{\text{ViewCount}} \times 100$$
4. **Transcript Scraping via `yt-dlp`**:
   * Runs: `yt-dlp --skip-download --write-auto-sub --sub-lang en --convert-subs vtt -o "workspace/{video_id}/transcript" "https://youtube.com/watch?v={video_id}"`.
   * Parses the WebVTT file into cleaned, chronological text segments discarding timestamp clutter.

---

### Step 3: Comment Extraction & Sentiment Analysis Engine

#### 1. Comment Scraper (`pkg/sentiment/comments.go`):
* Fetches the top 100 comments for the top 3 ranked viral videos using YouTube API `commentThreads.list`:
  * `Part: ["snippet"]`
  * `VideoId: videoID`
  * `Order: "relevance"` (YouTube's algorithmically upvoted comments)
  * `TextFormat: "plainText"`
* Fallback: When API quota is exhausted, falls back to `yt-dlp --write-comments --skip-download`.

#### 2. LLM Sentiment & Signal Extractor:
* The raw comments are fed to the fast LLM (`qwen2.5:7b` or `gpt-4o-mini`) with a strictly enforced JSON output schema:
```json
{
  "audience_sentiment": {
    "positive_triggers": ["What viewers loved about current explanations"],
    "negative_friction": ["What viewers found boring, confusing, or misleading"],
    "unanswered_questions": ["Key questions viewers asked in comments that the videos didn't answer"],
    "emotional_tone": "curiosity / skepticism / outrage / excitement",
    "target_audience_slang": ["niche keywords or slang words used repeatedly by commenters"],
    "debate_topics": ["Specific controversies that caused reply threads"]
  }
}
```

---

### Step 4: Scriptwriting & Pacing Synthesis LLM

#### 1. Input Context Injection
The Scriptwriter receives:
1. Target duration & format (e.g. 50 seconds Shorts, 9:16).
2. Transcript summaries of the top 3 viral videos (showing what structure worked).
3. The Comment Sentiment Report (identifying the curiosity gap to exploit).

#### 2. Generated Script Schema (`script.json`):
```json
{
  "title_idea": "The Secret Reason NASA Abandoned the Moon",
  "total_estimated_duration": 52,
  "hook": {
    "formula": "Negative Inversion + Curiosity Gap",
    "voiceover": "Everyone thinks NASA stopped going to the moon because of budget cuts. But that's only half the truth.",
    "visual_direction": "Dramatic fast zoom on black-and-white astronaut with red glitch overlay",
    "duration_sec": 4.5
  },
  "scenes": [
    {
      "scene_id": 1,
      "type": "kinetic_typography_with_data",
      "voiceover": "In 1972, Apollo 17 landed, packed up, and humans never returned. Why?",
      "visual_description": "Splitscreen: 1972 Apollo launch vs ticking 50-year digital clock counter",
      "animation_snippet_type": "SplitCounter",
      "sfx_cue": "ticking_clock_heavy_riser",
      "duration_sec": 5.0
    },
    {
      "scene_id": 2,
      "type": "chart_comparison",
      "voiceover": "At its peak, NASA ate over 4% of the entire US Federal budget. Today? Less than half of one percent.",
      "visual_description": "Animated descending bar chart with dramatic red falloff line",
      "animation_snippet_type": "BarChartDrop",
      "sfx_cue": "bass_drop",
      "duration_sec": 7.0
    }
  ],
  "cta": {
    "voiceover": "Which planet should humanity build a base on first? Drop it in the comments.",
    "visual_description": "Voting graphic: Mars vs Moon with pulse animation",
    "duration_sec": 4.0
  }
}
```

---

### Step 5: Parallel Generation Phase (4 Concurrent Tracks)

When `script.json` is generated, Go fans out 4 goroutines via `sync.WaitGroup`:

```
                ┌────────────────────────────────────────────────────────┐
                │             Goroutine Dispatcher (Go)                  │
                └───────┬──────────────┬──────────────┬──────────────┬───┘
                        │              │              │              │
                        ▼              ▼              ▼              ▼
                    Track 5A       Track 5B       Track 5C       Track 5D
                   (Voiceover)   (Media Code)   (Thumbnails)   (YouTube SEO)
```

#### Track 5A: Voiceover (TTS) + DeepFilterNet
1. **TTS Engine (`Kokoro-82M` / `Edge-TTS`)**:
   * Assembles all scene voiceovers into a master speech script with dynamic pause markers (`[pause 300ms]`).
   * Generates clean 48kHz WAV audio (`raw_voice.wav`).
2. **DeepFilterNet v3 Noise Compression**:
   * Invokes: `deep-filter raw_voice.wav -o workspace/{job_id}/ -m deepfilter3`.
   * **Why it matters**: Removes digital artifacts, floor room noise, pops, and stabilizes vocal dynamics to professional studio levels (`enhanced_voice.wav`).
3. **Word-Level Subtitle Alignment (WhisperX)**:
   * Generates word-level timestamps (`transcription.json`) mapping every word to an exact millisecond start and end time.

#### Track 5B: Media Generation (Code-Driven Visuals)
1. **Remotion (React/TypeScript) Setup**:
   * A pre-configured Remotion template exists in `web/remotion/`.
   * The Media Code LLM receives scene visual descriptions and generates the scene configuration file `scenes.json` or custom dynamic TSX components.
2. **Headless Video Rendering**:
   * Runs: `npx remotion render Root workspace/{job_id}/visuals.mp4 --props=workspace/{job_id}/props.json --concurrency=4`.
3. **Self-Healing Code Loop**:
   * If Remotion rendering crashes (syntax error, missing prop), the orchestrator catches `stderr`.
   * Sends the failed code + error stack back to the LLM: *"Fix this Remotion component error: [error]"*.
   * Retries rendering (up to 3 automated recovery cycles).

#### Track 5C: Fine-Tuned Thumbnail Generation
1. **Prompt Engineering for Thumbnails**:
   * Prompt Crafter LLM takes the Hook and Topic and designs a high-CTR visual formula:
     * *Focal point*: High-contrast subject (e.g. Astronaut looking terrified into darkness).
     * *Color scheme*: High-vibrancy complementary palette (Yellow/Black or Cyan/Orange).
     * *Rule*: No redundant titles on the image; max 3 punchy curiosity words.
2. **Image Synthesis via FLUX.1-schnell / SDXL**:
   * Dispatches generation to local ComfyUI REST API or local Diffusers service:
     * Model: `flux1-schnell.sft` / `sdxl_lightning.safetensors`.
     * Dimensions: `1280x720` (16:9) or `1080x1920` (Shorts cover).
3. **Post-Processing (ImageMagick / Go Imaging)**:
   * Increases unsharp mask by 15% and saturation by 8% to guarantee maximum pop on mobile screens.

#### Track 5D: YouTube SEO Model
1. Generates 3 Title candidates optimized for CTR:
   * Candidate 1: Curiosity-driven (*"NASA Finally Admitted Why We Never Went Back"*).
   * Candidate 2: High Search Volume (*"The Apollo 17 Mystery Explained"*).
   * Candidate 3: Contrarian (*"Why Returning to the Moon Was Cancelled"*).
2. Generates YouTube Description:
   * Includes keyword-rich summary (first 3 lines above the fold).
   * Generates video timestamps/chapters.
   * Relevant viral hashtags (`#space #nasa #apollo #science`).
3. Generates 15–20 high-relevance search tags.

---

### Step 6: FFmpeg Compositor & Audio Mastering

Once Track 5A and Track 5B complete, Go's FFmpeg pipeline performs the final assembly:

```bash
ffmpeg -y \
  -i workspace/{id}/visuals.mp4 \
  -i workspace/{id}/enhanced_voice.wav \
  -i assets/music/cinematic_ambient.mp3 \
  -filter_complex "\
    [2:a]volume=0.25[bgm]; \
    [bgm][1:a]sidechaincompress=threshold=0.08:ratio=6:attack=50:release=300[ducked_bgm]; \
    [1:a][ducked_bgm]amix=inputs=2:duration=first[audio_mix]; \
    [audio_mix]loudnorm=I=-14:LRA=7:tp=-1.5[final_audio]; \
    [0:v]subtitles=workspace/{id}/captions.ass[final_video]" \
  -map "[final_video]" \
  -map "[final_audio]" \
  -c:v libx264 -preset fast -crf 18 -pix_fmt yuv420p \
  -c:a aac -b:a 192k \
  output/{id}/final_render.mp4
```

* **Dynamic Subtitles (`captions.ass`)**: Built from WhisperX word timestamps. Animated bounce effects, uppercase bold font, yellow highlighting for active spoken words.
* **Sidechain Audio Ducking**: Background music automatically drops by 14dB whenever speech audio is detected and smoothly recovers during pauses.
* **Loudness Normalization**: Mastered to `-14 LUFS` to prevent YouTube's automatic volume attenuation penalty.

---

### Step 7: Go Web Frontend & Hosted Dashboard

A self-hosted management interface embedded directly into the Gozart executable.

#### Technology Stack:
* **Backend**: Go `net/http` + `github.com/go-chi/chi/v5` router.
* **Templating**: Go native `html/template` embedded with `//go:embed web/templates/*`.
* **Frontend Reactivity**: **HTMX (v1.9.10)** (handles dynamic tab switching, AJAX form submission, and SSE streaming).
* **Styling**: Tailwind CSS (compiled standalone, zero Node runtime needed at serve time).
* **Live Updates**: Server-Sent Events (SSE) via `/api/jobs/{id}/stream` sending real-time step progress and terminal logs directly to the browser.

#### Web Interface Screens:
1. **Dashboard (`/`)**:
   * Summary cards: Total Videos Generated, Uploaded Videos, Active Background Jobs.
   * Table of recent jobs with status badges: `QUEUED`, `RESEARCHING`, `GENERATING`, `COMPLETED`, `FAILED`.
2. **New Video Creator (`/new`)**:
   * Form: Topic Idea, Country Selector, Duration Slider, Tone Selector, Model Backend toggle.
   * *"Launch Pipeline"* button triggers background goroutine job and redirects to the Job Inspector.
3. **Live Job Inspector (`/jobs/{id}`)**:
   * **Stage Progress Bar**: Visual stepper highlighting the active stage in real time.
   * **Terminal Logs Tab**: Live SSE stream of FFmpeg, yt-dlp, and Ollama output.
   * **Audience Insights Tab**: Visual presentation of comment sentiment, top objections, and questions extracted from competitors.
   * **Script Inspector Tab**: Formatted scene-by-scene script cards with audio cues.
   * **Preview & Publish Section**:
     * In-browser HTML5 `<video>` player for `final_render.mp4`.
     * Thumbnail comparison preview.
     * Title & Description editor.
     * One-click *"Publish to YouTube"* button.
4. **Settings (`/settings`)**:
   * YouTube OAuth2 authentication status & re-authorize button.
   * API Key configuration (Ollama URL, YouTube API Key, ElevenLabs, ComfyUI).

---

### Step 8: Database & State Machine Checkpointing

To ensure zero lost work if a render fails or power cuts out, Gozart uses an **embedded SQLite database** (`modernc.org/sqlite`).

#### Schema (`pkg/storage/schema.sql`):
```sql
CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    country_code TEXT NOT NULL,
    format TEXT NOT NULL,
    status TEXT NOT NULL,
    current_step TEXT NOT NULL,
    progress_pct INTEGER DEFAULT 0,
    script_json TEXT,
    sentiment_json TEXT,
    seo_json TEXT,
    video_path TEXT,
    thumbnail_path TEXT,
    youtube_video_id TEXT,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS job_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(job_id) REFERENCES jobs(id)
);
```

#### Checkpointing States:
* `INIT` $\rightarrow$ `RESEARCH_DONE` $\rightarrow$ `SENTIMENT_DONE` $\rightarrow$ `SCRIPT_DONE` $\rightarrow$ `PARALLEL_GEN_DONE` $\rightarrow$ `COMPOSITED` $\rightarrow$ `UPLOADED`.
* If the process crashes during media generation, restarting Gozart automatically resumes from `SCRIPT_DONE` without re-running YouTube search or burning LLM tokens.

---

## 5. Project Directory Structure

```
gozart/
├── cmd/
│   ├── clean.go             # Cache and artifact cleaner
│   ├── process.go           # CLI pipeline processor
│   ├── root.go              # Root Cobra command
│   └── web.go               # Web server command (`gozart web`)
├── config.json              # Main runtime configuration
├── go.mod
├── go.sum
├── main.go                  # Entrypoint
├── pkg/
│   ├── agent/               # AI Agent & Tool Calling Harness
│   │   ├── agent.go         # Ollama / LLM agent loop
│   │   ├── tools.go         # [search-youtube-videos] tool definition
│   │   └── prompts.go       # System prompts for research & scriptwriting
│   ├── audio/               # Voice & Audio Production
│   │   ├── deepfilter.go    # DeepFilterNet integration
│   │   ├── tts.go           # Kokoro & Edge-TTS client
│   │   └── whisper.go       # Word-level timestamp alignment
│   ├── compositor/          # FFmpeg Pipeline
│   │   ├── compositor.go    # Audio ducking, loudness normalization, subtitle burn-in
│   │   └── subtitles.go     # .ass dynamic subtitle generator
│   ├── config/              # Configuration loader & validator
│   ├── downloader/          # yt-dlp wrapper & transcript extractor
│   ├── image/               # Thumbnail Generation
│   │   ├── comfyui.go       # FLUX.1 / SDXL ComfyUI client
│   │   └── enhancer.go      # Contrast & sharpness filters
│   ├── logger/              # Thread-safe logging
│   ├── media/               # Code-to-Media Engine
│   │   ├── remotion.go      # Remotion headless executor
│   │   ├── selfheal.go      # Self-healing compilation retry loop
│   │   └── manim.go         # Manim CE alternative executor
│   ├── research/            # YouTube API Research & Outlier Velocity
│   │   ├── metrics.go       # Velocity & V/S scoring algorithms
│   │   └── youtube.go       # YouTube Data API v3 client
│   ├── sentiment/           # Comment Sentiment & Signal Extraction
│   │   ├── comments.go      # Comment scraper
│   │   └── extractor.go     # LLM sentiment extraction
│   ├── seo/                 # YouTube SEO Optimization
│   │   └── seo.go           # Title, description, tag generator
│   ├── storage/             # SQLite State Machine & Checkpointing
│   │   ├── db.go            # SQLite connection & migration
│   │   └── jobs.go          # Job CRUD & resume logic
│   └── uploader/            # YouTube API Upload & OAuth2
│       └── uploader.go      # Video + Thumbnail publisher
├── web/                     # Hosted Go Template Frontend
│   ├── embed.go             # Go //go:embed declaration
│   ├── handlers/            # HTTP & SSE handlers
│   │   ├── dashboard.go     # Dashboard view
│   │   ├── jobs.go          # Job creation & inspection
│   │   └── sse.go           # Server-Sent Events log streamer
│   ├── static/              # Static assets
│   │   ├── css/
│   │   │   └── tailwind.css # Compiled Tailwind styles
│   │   └── js/
│   │       └── htmx.min.js  # Embedded HTMX library
│   └── templates/           # Go HTML Templates
│       ├── base.html        # Main layout
│       ├── dashboard.html   # Job list & statistics
│       ├── job_view.html    # Live inspector & video player
│       └── new_job.html     # Idea submission form
├── workflow.excalidraw      # Visual architecture workflow diagram
└── plan.md                  # This master specification document
```

---

## 6. Phase-by-Phase Implementation Roadmap

### Phase 1: Core Storage, Database & Web Dashboard Skeleton
* Initialize SQLite database and job checkpointing models.
* Create `gozart web` Cobra command with `chi` router and embedded templates.
* Build the HTMX-driven dashboard, job creation form, and live SSE log streamer.

### Phase 2: AI Agent & Research Tool (`search-youtube-videos`)
* Implement the YouTube Data API v3 search client with `regionCode` filtering.
* Build velocity scoring ($V_v$) and the `yt-dlp` subtitle downloader.
* Wire the `search-youtube-videos` tool into the Ollama tool-calling harness.

### Phase 3: Comment Sentiment & Scriptwriting Engine
* Implement the YouTube comment scraper and LLM sentiment classification prompt.
* Implement the structured scriptwriter generating scene-by-scene JSON schemas.

### Phase 4: Parallel Audio & Media Code Engines
* Integrate TTS (`Kokoro-82M` / `Edge-TTS`) and compile the `DeepFilterNet` audio polish step.
* Build the Remotion execution engine and implement the Self-Healing Code Error loop.

### Phase 5: Thumbnails, SEO & FFmpeg Compositor
* Connect FLUX.1 / ComfyUI API for thumbnail generation.
* Build the FFmpeg sidechain ducking and loudness normalization pipeline.
* Implement the YouTube SEO generator (Title, Description, Tags).

### Phase 6: End-to-End Integration & YouTube Publishing
* Connect the final video, thumbnail, and SEO metadata to `pkg/uploader`.
* Conduct end-to-end autonomous runs from web submission to published YouTube video.
* Implement error telemetry and automatic checkpoint recovery.
