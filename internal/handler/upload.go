package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsCfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/config"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
)

type UploadHandler struct {
	client    *s3.Client
	bucket    string
	publicURL string
}

func NewUploadHandler(cfg *config.Config) (*UploadHandler, error) {
	awsConfig, err := awsCfg.LoadDefaultConfig(context.Background(),
		awsCfg.WithRegion("auto"),
		awsCfg.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.R2AccessKey, cfg.R2SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.R2Endpoint)
		o.UsePathStyle = true
	})

	log.Printf("Upload handler initialized: bucket=%s, endpoint=%s", cfg.R2Bucket, cfg.R2Endpoint)

	return &UploadHandler{
		client:    client,
		bucket:    cfg.R2Bucket,
		publicURL: cfg.R2PublicURL,
	}, nil
}

func (h *UploadHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	log.Printf("UploadFile: starting upload request, content-length=%d", r.ContentLength)

	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Printf("UploadFile: parse multipart form failed: %v", err)
		writeErr(w, http.StatusBadRequest, "File too large or invalid form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("UploadFile: no file in form: %v", err)
		writeErr(w, http.StatusBadRequest, "No file provided")
		return
	}
	defer file.Close()

	log.Printf("UploadFile: received file name=%s, size=%d, content-type=%s",
		header.Filename, header.Size, header.Header.Get("Content-Type"))

	if header.Size > 50<<20 {
		log.Printf("UploadFile: file too large: %d bytes", header.Size)
		writeErr(w, http.StatusBadRequest, "File exceeds 50MB limit")
		return
	}

	ext := ""
	for i := len(header.Filename) - 1; i >= 0; i-- {
		if header.Filename[i] == '.' {
			ext = header.Filename[i:]
			break
		}
	}

	key := uuid.New().String() + ext
	log.Printf("UploadFile: generated object key=%s", key)

	body, err := io.ReadAll(file)
	if err != nil {
		log.Printf("UploadFile: failed to read file body: %v", err)
		writeErr(w, http.StatusInternalServerError, "Failed to read file")
		return
	}
	log.Printf("UploadFile: read %d bytes from file", len(body))

	log.Printf("UploadFile: uploading to S3 bucket=%s, key=%s", h.bucket, key)
	_, err = h.client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(h.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(header.Header.Get("Content-Type")),
	})
	if err != nil {
		log.Printf("UploadFile: S3 PutObject FAILED: %v", err)
		writeErr(w, http.StatusInternalServerError, "Failed to upload file")
		return
	}

	log.Printf("UploadFile: upload successful, url=%s/%s", h.publicURL, key)

	writeJSON(w, http.StatusOK, model.UploadResponse{
		URL:  h.publicURL + "/" + key,
		Name: header.Filename,
		Type: header.Header.Get("Content-Type"),
		Size: header.Size,
	})
}


