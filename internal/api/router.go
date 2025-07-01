package api

import (
    "github.com/gin-gonic/gin"
    "github.com/leozw/uptime-guardian/internal/api/handlers"
    "github.com/leozw/uptime-guardian/internal/api/middleware"
    "github.com/leozw/uptime-guardian/internal/config"
    "github.com/leozw/uptime-guardian/internal/queue"
    "github.com/leozw/uptime-guardian/internal/storage/postgres"
    "github.com/leozw/uptime-guardian/internal/storage/redis"
)

type Server struct {
    Config  *config.Config
    Router  *gin.Engine
    DB      *postgres.DB
    Cache   *redis.Client
    Queue   *queue.RedisQueue
}

func NewServer(cfg *config.Config, db *postgres.DB, cache *redis.Client, queue *queue.RedisQueue) *Server {
    gin.SetMode(cfg.GinMode)
    router := gin.New()
    
    // Middleware
    router.Use(gin.Logger())
    router.Use(gin.Recovery())
    router.Use(middleware.CORS())
    
    server := &Server{
        Config: cfg,
        Router: router,
        DB:     db,
        Cache:  cache,
        Queue:  queue,
    }
    
    server.setupRoutes()
    return server
}

func (s *Server) setupRoutes() {
    // Health check
    s.Router.GET("/health", handlers.HealthCheck)
    
    // Auth routes
    authHandler := handlers.NewAuthHandler(s.DB, s.Config)
    auth := s.Router.Group("/api/v1/auth")
    {
        auth.POST("/signup", authHandler.SignUp)
        auth.POST("/login", authHandler.Login)
        auth.POST("/refresh", authHandler.RefreshToken)
    }
    
    // API routes (protected)
    api := s.Router.Group("/api/v1")
    api.Use(middleware.AuthRequired(s.Config.JWTSecret))
    api.Use(middleware.TenantContext(s.DB))
    
    // Domain routes
    domainHandler := handlers.NewDomainHandler(s.DB, s.Queue)
    {
        api.GET("/domains", domainHandler.ListDomains)
        api.POST("/domains", domainHandler.CreateDomain)
        api.GET("/domains/:id", domainHandler.GetDomain)
        api.PUT("/domains/:id", domainHandler.UpdateDomain)
        api.DELETE("/domains/:id", domainHandler.DeleteDomain)
        api.GET("/domains/:id/health", domainHandler.GetDomainHealth)
        api.POST("/domains/:id/check", domainHandler.TriggerCheck)
    }
    
    // Metrics routes
    metricsHandler := handlers.NewMetricsHandler(s.DB)
    {
        api.GET("/metrics/overview", metricsHandler.GetOverview)
        api.GET("/metrics/domains/:id", metricsHandler.GetDomainMetrics)
    }
}