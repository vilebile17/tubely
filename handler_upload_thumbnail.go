package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

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

	const maxMemory = 10 << 20
	if err = r.ParseMultipartForm(maxMemory); err != nil {
		respondWithError(w, http.StatusBadRequest, "There was an error parsing the request", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "There was an error forming the file", err)
		return
	}
	defer file.Close()

	contentTypeHeader := header.Header.Get("Content-Type")
	if contentTypeHeader == "" {
		respondWithError(w, http.StatusBadRequest, "The Content-Type header is empty", nil)
		return
	}

	mediaType, _, err := mime.ParseMediaType(contentTypeHeader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "There was an error parsing the contentHeader", err)
		return
	}
	if mediaType != "image/jpeq" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "The thumbnail must be a png or a jpeg", nil)
		return
	}
	diskPath := filepath.Join(cfg.assetsRoot, videoID.String())
	diskPath += "." + strings.Split(mediaType, "/")[1]

	dst, err := os.Create(diskPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "There was an error creating the new file...", err)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "There was an error copying the image to the new file...", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "There was an error retrieving the video data from the database", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You can't upload the thumbnail to a video that you don't own...", nil)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%v/assets/%v.png", cfg.port, videoID)
	video.ThumbnailURL = &thumbnailURL
	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusBadRequest, "There was an error adding the video data from the database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
