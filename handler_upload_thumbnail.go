package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
)

const maxMemory = 10 << 20
const thumnailFileKey = "thumbnail"

var acceptedThumbnailMimeTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
}

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	if err := r.ParseMultipartForm(maxMemory); err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse upload", err)
		return
	}
	file, _, err := r.FormFile(thumnailFileKey)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse upload", err)
		return
	}
	mtype, err := mimetype.DetectReader(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse upload", err)
		return
	}
	if !acceptedThumbnailMimeTypes[mtype.String()] {
		respondWithError(w, http.StatusBadRequest, "Thumbnail must be jpeg or png", nil)
		return
	}
	file.Seek(0, io.SeekStart)
	extension := mtype.Extension()
	randomFileName := make([]byte, 32)
	rand.Read(randomFileName)
	fileName := fmt.Sprintf("%s%s", base64.RawURLEncoding.EncodeToString(randomFileName), extension)
	path := filepath.Join(cfg.assetsRoot, fileName)
	videoMeta, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse upload", err)
		return
	}
	if videoMeta.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User does not own targeted video", err)
		return
	}

	target, err := os.Create(path)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}
	if _, err := io.Copy(target, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileName)
	videoMeta.ThumbnailURL = &thumbnailURL
	if err = cfg.db.UpdateVideo(videoMeta); err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse upload", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMeta)
}
