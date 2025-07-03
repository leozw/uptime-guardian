package handlers

import (
	"github.com/leozw/uptime-guardian/internal/db"
	"github.com/leozw/uptime-guardian/internal/metrics"
	"github.com/leozw/uptime-guardian/pkg/keycloak"
	"go.uber.org/zap"
)

type Handler struct {
	repo     *db.Repository
	metrics  *metrics.Collector
	keycloak *keycloak.Client
	logger   *zap.Logger
}

func NewHandler(repo *db.Repository, metrics *metrics.Collector, keycloak *keycloak.Client, logger *zap.Logger) *Handler {
	return &Handler{
		repo:     repo,
		metrics:  metrics,
		keycloak: keycloak,
		logger:   logger,
	}
}
