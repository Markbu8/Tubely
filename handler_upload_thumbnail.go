package main

import (
	"fmt"
	"io"
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

	const maxMemory = 10 << 20 // 10 MB

	if err := r.ParseMultipartForm(maxMemory); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body", err)
	}

	thumbMultiFile, thumbHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	defer thumbMultiFile.Close()

	//Gat video metadata from database
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error retrieving video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Access denied", nil)

		return
	}

	//add thumbnail to assets folder from multipart.file
	//get Image file type
	partsThumbHeader := strings.Split(thumbHeader.Header.Get("Content-Type"), "/")
	if len(partsThumbHeader) < 2 {
		respondWithError(w, http.StatusBadRequest, "Invalid Content Type header", nil)
		return
	} else if partsThumbHeader[0] != "image" {
		respondWithError(w, http.StatusBadRequest, "Not Image Type", nil)
		return
	}

	//Create file in assets folder and write image data to it
	thumbUrlPath := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%s.%s", video.ID, partsThumbHeader[1]))
	thumbFile, err := os.Create(thumbUrlPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating video file", err)
		return
	}
	defer thumbFile.Close()

	_, err = io.Copy(thumbFile, thumbMultiFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing video file", err)
	}

	//Update video thumbnail URL in database and respond with thumbnail URL
	thumbURL := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, video.ID, partsThumbHeader[1])
	video.ThumbnailURL = &thumbURL

	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
