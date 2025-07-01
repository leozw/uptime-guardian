package config

import (
    "os"
    "strconv"
    "time"
)

type Config struct {
    // Server
    Port    string
    GinMode string
    
    // Database
    DatabaseURL string
    
    // Redis
    RedisURL string
    
    // JWT
    JWTSecret string
    
    // Mimir
    MimirURL string
    
    // Worker
    WorkerCount int
    
    // Timeouts
    CheckTimeout time.Duration
}

func Load() *Config {
    return &Config{
        Port:         getEnv("PORT", "8080"),
        GinMode:      getEnv("GIN_MODE", "release"),
        DatabaseURL:  getEnv("DATABASE_URL", "postgres://localhost/domainmonitor"),
        RedisURL:     getEnv("REDIS_URL", "redis://localhost:6379"),
        JWTSecret:    getEnv("JWT_SECRET", "change-me-in-production"),
        MimirURL:     getEnv("MIMIR_URL", "http://localhost:9009"),
        WorkerCount:  getEnvAsInt("WORKER_COUNT", 5),
        CheckTimeout: getEnvAsDuration("CHECK_TIMEOUT", "30s"),
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
    valueStr := os.Getenv(key)
    if value, err := strconv.Atoi(valueStr); err == nil {
        return value
    }
    return defaultValue
}

func getEnvAsDuration(key string, defaultValue string) time.Duration {
    valueStr := getEnv(key, defaultValue)
    if duration, err := time.ParseDuration(valueStr); err == nil {
        return duration
    }
    
    // Fallback to default
    duration, _ := time.ParseDuration(defaultValue)
    return duration
}