package controlplane

import (
	"context"
	"os"
	"strings"

	"github.com/redis/agent-filesystem/internal/queryembedding"
)

type QueryModelStatusRequest struct {
	Model string `json:"model,omitempty"`
}

type QueryModelDownloadRequest struct {
	Model string `json:"model,omitempty"`
}

type QueryModelStatus = queryembedding.LocalModelStatus
type QueryModelDownloadResult = queryembedding.LocalModelDownloadResult

func (s *Service) QueryModelStatus(ctx context.Context, request QueryModelStatusRequest) (QueryModelStatus, error) {
	_ = ctx
	return queryembedding.ResolveLocalModelStatus(queryModelRequestModel(request.Model), "")
}

func (m *DatabaseManager) QueryModelStatus(ctx context.Context, request QueryModelStatusRequest) (QueryModelStatus, error) {
	_ = m
	service := &Service{}
	return service.QueryModelStatus(ctx, request)
}

func (s *Service) DownloadQueryModel(ctx context.Context, request QueryModelDownloadRequest) (QueryModelDownloadResult, error) {
	return queryembedding.WarmLocalGGUFModel(ctx, queryModelRequestModel(request.Model))
}

func (m *DatabaseManager) DownloadQueryModel(ctx context.Context, request QueryModelDownloadRequest) (QueryModelDownloadResult, error) {
	_ = m
	service := &Service{}
	return service.DownloadQueryModel(ctx, request)
}

func queryModelRequestModel(model string) string {
	model = strings.TrimSpace(model)
	if model != "" {
		return model
	}
	return strings.TrimSpace(os.Getenv("AFS_EMBED_MODEL"))
}
