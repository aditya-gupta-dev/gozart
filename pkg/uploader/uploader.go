package uploader

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"gozart/pkg/config"
	"gozart/pkg/logger"
)

type YouTubeUploader struct {
	logger           *logger.Logger
	configLoader     *config.ConfigLoader
	clientSecretFile string
	tokenFile        string
	Service          *youtube.Service
}

func New(l *logger.Logger, cfg *config.ConfigLoader) *YouTubeUploader {
	return &YouTubeUploader{
		logger:           l,
		configLoader:     cfg,
		clientSecretFile: "client_secrets.json",
		tokenFile:        "token.json",
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
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		fmt.Printf("Unable to read authorization code: %v\n", err)
		os.Exit(1)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
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

// ProgressReader wraps a file to track upload progress
type ProgressReader struct {
	file *os.File
	bar  *progressbar.ProgressBar
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.file.Read(p)
	pr.bar.Add(n)
	return n, err
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
	
	bar := progressbar.DefaultBytes(stat.Size(), "Uploading")
	
	// Create a reader that updates the progress bar
	progressReader := &ProgressReader{
		file: file,
		bar:  bar,
	}

	response, err := call.Media(progressReader).Do()
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
