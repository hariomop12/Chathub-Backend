package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsCfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/config"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/httpapi"
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

	slog.Info("upload handler initialized", "bucket", cfg.R2Bucket, "endpoint", cfg.R2Endpoint)

	return &UploadHandler{
		client:    client,
		bucket:    cfg.R2Bucket,
		publicURL: cfg.R2PublicURL,
	}, nil
}

func (h *UploadHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	slog.Info("upload request", "content_length", r.ContentLength)

	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		slog.Warn("parse multipart form failed", "error", err)
		httpapi.WriteErr(w, http.StatusBadRequest, "PAYLOAD_TOO_LARGE", "File too large or invalid form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		slog.Warn("no file in form", "error", err)
		httpapi.WriteErr(w, http.StatusBadRequest, "", "No file provided")
		return
	}
	defer file.Close()

	slog.Info("upload file received", "name", header.Filename, "size", header.Size, "content_type", header.Header.Get("Content-Type"))

	if header.Size > 50<<20 {
		slog.Warn("file too large", "size", header.Size)
		httpapi.WriteErr(w, http.StatusBadRequest, "PAYLOAD_TOO_LARGE", "File exceeds 50MB limit")
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
	slog.Info("generated object key", "key", key)

	body, err := io.ReadAll(file)
	if err != nil {
		slog.Warn("failed to read file body", "error", err)
		httpapi.WriteErr(w, http.StatusInternalServerError, "", "Failed to read file")
		return
	}
	slog.Info("read file bytes", "size", len(body))

	slog.Info("uploading to s3", "bucket", h.bucket, "key", key)
	_, err = h.client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(h.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(header.Header.Get("Content-Type")),
	})
	if err != nil {
		slog.Error("s3 put object failed", "error", err)
		httpapi.WriteErr(w, http.StatusInternalServerError, "", "Failed to upload file")
		return
	}

	slog.Info("upload successful", "url", h.publicURL+"/"+key)

	httpapi.WriteJSON(w, http.StatusOK, model.UploadResponse{
		URL:  h.publicURL + "/" + key,
		Name: header.Filename,
		Type: header.Header.Get("Content-Type"),
		Size: header.Size,
	})
}
