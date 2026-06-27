package uploader

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"gozart/pkg/config"
	"gozart/pkg/logger"
)

type YouTubeUploader struct {
	logger            *logger.Logger
	configLoader      *config.ConfigLoader
	clientSecretFile  string
	tokenFile         string
	Service           *youtube.Service
	ProgressContainer *mpb.Progress
}

func New(l *logger.Logger, cfg *config.ConfigLoader) *YouTubeUploader {
	return &YouTubeUploader{
		logger:            l,
		configLoader:      cfg,
		clientSecretFile:  "client_secrets.json",
		tokenFile:         "token.json",
		ProgressContainer: mpb.New(mpb.WithWidth(60)),
	}
}

func (u *YouTubeUploader) Wait() {
	if u.ProgressContainer != nil {
		u.ProgressContainer.Wait()
	}
}

func (u *YouTubeUploader) Authenticate() {
	ctx := context.Background()

	b, err := os.ReadFile(u.clientSecretFile)
	if err != nil {
		u.logger.LogFileWithStdout("Unable to read client secret file: "+err.Error(), logger.Fatal)
	}

	config, err := google.ConfigFromJSON(b, youtube.YoutubeUploadScope)
	if err != nil {
		u.logger.LogFileWithStdout("Unable to parse client secret file to config: "+err.Error(), logger.Fatal)
	}

	client := getClient(config, u.tokenFile, u.logger)

	service, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		u.logger.LogFileWithStdout("Error creating YouTube client: "+err.Error(), logger.Fatal)
	}

	u.Service = service
	u.logger.LogFileWithStdout("Successfully authenticated with YouTube API", logger.Info)
}

func getClient(config *oauth2.Config, tokenFile string, l *logger.Logger) *http.Client {
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokenFile, tok, l)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	// Bind the loopback IP literally rather than the hostname "localhost".
	// "localhost" resolves to both 127.0.0.1 and ::1, and Go listens on only
	// one of them; the browser may then try the other family and get a
	// connection refused ("cannot access localhost"). Google also recommends
	// 127.0.0.1 over localhost for loopback redirect URIs.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Printf("Unable to start local server: %v\n", err)
		os.Exit(1)
	}

	// Derive the redirect URL from the actual bound address so the host:port
	// the browser is sent to always matches what we're listening on.
	config.RedirectURL = fmt.Sprintf("http://%s", listener.Addr().String())
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser to authorize the application: \n%v\n", authURL)

	ch := make(chan string)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			fmt.Fprintf(w, "Authentication successful! You can close this window.")
			ch <- code
		} else {
			fmt.Fprintf(w, "Authentication failed. No code found.")
			ch <- ""
		}
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	authCode := <-ch
	srv.Shutdown(context.Background())

	if authCode == "" {
		fmt.Println("Unable to read authorization code")
		os.Exit(1)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		fmt.Printf("Unable to retrieve token from web: %v\n", err)
		os.Exit(1)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token, l *logger.Logger) {
	l.LogFileWithStdout(fmt.Sprintf("Saving credential file to: %s\n", path), logger.Info)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		l.LogFileWithStdout("Unable to cache oauth token: "+err.Error(), logger.Fatal)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}



func (u *YouTubeUploader) UploadVideo(videoFile, title, description string, tags []string) string {
	if u.Service == nil {
		u.logger.LogFileWithStdout("Not authenticated. Call authenticate() first.", logger.Fatal)
	}

	file, err := os.Open(videoFile)
	if err != nil {
		u.logger.LogFileWithStdout("Video file not found: "+videoFile, logger.Error)
		return ""
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		u.logger.LogFileWithStdout("Could not stat video file: "+videoFile, logger.Error)
		return ""
	}

	upload := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       title,
			Description: description,
			Tags:        tags,
			CategoryId:  "22", // People & Blogs
		},
		Status: &youtube.VideoStatus{PrivacyStatus: "private"},
	}

	call := u.Service.Videos.Insert([]string{"snippet", "status"}, upload)
	
	bar := u.ProgressContainer.AddBar(stat.Size(),
		mpb.PrependDecorators(
			decor.Name(title, decor.WCSyncSpaceR),
			decor.CountersKibiByte("% .2f / % .2f", decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WCSyncSpace),
		),
	)

	proxyReader := bar.ProxyReader(file)
	defer proxyReader.Close()

	response, err := call.Media(proxyReader).Do()
	if err != nil {
		u.logger.LogFileWithStdout("Error making YouTube API call: "+err.Error(), logger.Error)
		return ""
	}

	u.logger.LogFileWithStdout("Video uploaded successfully. Video ID: "+response.Id, logger.Info)
	return response.Id
}

func (u *YouTubeUploader) UploadThumbnail(videoID, thumbnailPath string) {
	file, err := os.Open(thumbnailPath)
	if err != nil {
		u.logger.LogFileWithStdout("Thumbnail file not found: "+thumbnailPath, logger.Error)
		return
	}
	defer file.Close()

	call := u.Service.Thumbnails.Set(videoID)
	_, err = call.Media(file).Do()
	if err != nil {
		u.logger.LogFileWithStdout("Error uploading thumbnail: "+err.Error(), logger.Error)
		return
	}
	u.logger.LogFileWithStdout("Thumbnail uploaded successfully!", logger.Info)
}
