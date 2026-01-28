package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/video"
	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
)

const maxUploadSize = 1 >> 30
const videoKey = "video"
const tempFilename = "*tubely-upload.mp4"

var acceptedVideolMimeTypes = map[string]bool{
	"video/mp4": true,
}

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	http.MaxBytesReader(w, r.Body, maxUploadSize)
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	videoMeta, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse upload", err)
		return
	}
	if videoMeta.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User does not own targeted video", err)
		return
	}
	v, _, err := r.FormFile(videoKey)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse upload", err)
		return
	}
	defer v.Close()
	mtype, err := mimetype.DetectReader(v)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse upload", err)
		return
	}
	mimeType := mtype.String()
	if !acceptedVideolMimeTypes[mimeType] {
		respondWithError(w, http.StatusBadRequest, "Video must be mp4", nil)
		return
	}
	v.Seek(0, io.SeekStart)

	// save to temp file
	tempFile, err := os.CreateTemp("", tempFilename)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	if _, err := io.Copy(tempFile, v); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}
	tempFile.Seek(0, io.SeekStart)

	ratio, err := video.GetVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to read metadata from video", err)
		return
	}
	var prefix string
	switch ratio {
	case video.SixteenByNine:
		prefix = "landscape"
	case video.NineBySixteen:
		prefix = "portrait"
	default:
		prefix = "other"
	}

	randomFileName := make([]byte, 32)
	rand.Read(randomFileName)
	key := fmt.Sprintf("%s/%s%s", prefix, base64.RawURLEncoding.EncodeToString(randomFileName), mtype.Extension())

	params := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        tempFile,
		ContentType: &mimeType,
	}

	if _, err := cfg.s3Client.PutObject(r.Context(), &params); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}
	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", *params.Bucket, cfg.s3Region, key)
	videoMeta.VideoURL = &videoURL
	if err = cfg.db.UpdateVideo(videoMeta); err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse upload", err)
		return
	}
	respondWithJSON(w, http.StatusOK, videoMeta)
}
