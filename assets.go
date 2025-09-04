package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) string {
	base := make([]byte, 32)
	_, err := rand.Read(base)
	if err != nil {
		panic("failed to generate random bytes")
	}
	id := base64.RawURLEncoding.EncodeToString(base)

	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", id, ext)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("https://%s/%s", cfg.s3CfDistribution, assetPath)
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func getVideoAspectRatio(filePath string) (string, error) {

	type ffprobeOutput struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}

	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out

	// Run ffprobe
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse JSON output
	var probe ffprobeOutput
	if err := json.Unmarshal(out.Bytes(), &probe); err != nil {
		return "", fmt.Errorf("failed to unmarshal ffprobe output: %w", err)
	}

	// Find the first stream with width/height
	var width, height int
	for _, stream := range probe.Streams {
		if stream.Width > 0 && stream.Height > 0 {
			width, height = stream.Width, stream.Height
			break
		}
	}

	if width == 0 || height == 0 {
		return "other", nil // could not detect
	}

	// Calculate aspect ratio
	ratio := float64(width) / float64(height)

	// Allow a little tolerance (e.g. 1.77 ~= 16:9)
	if math.Abs(ratio-16.0/9.0) < 0.05 {
		return "16:9", nil
	}
	if math.Abs(ratio-9.0/16.0) < 0.05 {
		return "9:16", nil
	}
	return "other", nil
}

func processVideoForFastStart(filePath string) (string, error) {
	outputPath := fmt.Sprintf("%s.processing.mp4", filePath)

	cmd := exec.Command(
		"ffmpeg",
		"-i", filePath,
		"-c", "copy",
		"-movflags", "faststart",
		"-f", "mp4",
		outputPath,
	)

	// Capture stderr in case ffmpeg errors out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg failed: %v, %s", err, stderr.String())
	}

	return outputPath, nil
}

//func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
//	// Create a presign client
//	presignClient := s3.NewPresignClient(s3Client)
//
//	// Generate the presigned URL
//	result, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
//		Bucket: aws.String(bucket),
//		Key:    aws.String(key),
//	}, s3.WithPresignExpires(expireTime))
//	if err != nil {
//		return "", err
//	}
//
//	// Return the actual URL string
//	return result.URL, nil
//}
//
//func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
//	if video.VideoURL == nil {
//		return video, nil
//	}
//	// Split "bucket,key"
//	parts := strings.Split(*video.VideoURL, ",")
//	if len(parts) != 2 {
//		return video, fmt.Errorf("invalid VideoURL format, expected 'bucket,key'")
//	}
//	bucket := parts[0]
//	key := parts[1]
//
//	// Generate presigned URL
//	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, 10*time.Minute)
//	if err != nil {
//		return video, err
//	}
//
//	// Replace VideoURL with presigned one
//	video.VideoURL = &presignedURL
//	return video, nil
//}
