package handlers

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
    
    "github.com/leozw/uptime-guardian/internal/config"
    "github.com/leozw/uptime-guardian/internal/core"
    "github.com/leozw/uptime-guardian/internal/storage/postgres"
)

type AuthHandler struct {
    db     *postgres.DB
    config *config.Config
}

func NewAuthHandler(db *postgres.DB, config *config.Config) *AuthHandler {
    return &AuthHandler{db: db, config: config}
}

type SignUpRequest struct {
    Name     string `json:"name" binding:"required"`
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
    Token        string       `json:"token"`
    RefreshToken string       `json:"refresh_token"`
    Tenant       *core.Tenant `json:"tenant"`
}

func (h *AuthHandler) SignUp(c *gin.Context) {
    var req SignUpRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Check if email exists
    exists, err := h.db.EmailExists(req.Email)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check email"})
        return
    }
    if exists {
        c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
        return
    }

    // Hash password
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
        return
    }

    // Create tenant
    tenant := &core.Tenant{
        ID:               uuid.New(),
        Name:             req.Name,
        Email:            req.Email,
        APIKey:           generateAPIKey(),
        MimirTenantID:    uuid.New().String(),
        MaxDomains:       10, // Free tier
        CheckIntervalMin: 5,
        IsActive:         true,
        CreatedAt:        time.Now(),
        UpdatedAt:        time.Now(),
    }

    // Save tenant with password
    if err := h.db.CreateTenant(tenant, string(hashedPassword)); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
        return
    }

    // Generate tokens
    token, refreshToken, err := h.generateTokens(tenant.ID.String())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
        return
    }

    c.JSON(http.StatusCreated, AuthResponse{
        Token:        token,
        RefreshToken: refreshToken,
        Tenant:       tenant,
    })
}

func (h *AuthHandler) Login(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Get tenant by email
    tenant, hashedPassword, err := h.db.GetTenantByEmail(req.Email)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
        return
    }

    // Check password
    if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
        return
    }

    // Check if active
    if !tenant.IsActive {
        c.JSON(http.StatusForbidden, gin.H{"error": "Account is disabled"})
        return
    }

    // Generate tokens
    token, refreshToken, err := h.generateTokens(tenant.ID.String())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
        return
    }

    c.JSON(http.StatusOK, AuthResponse{
        Token:        token,
        RefreshToken: refreshToken,
        Tenant:       tenant,
    })
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
    var req struct {
        RefreshToken string `json:"refresh_token" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Validate refresh token
    claims := &jwt.RegisteredClaims{}
    token, err := jwt.ParseWithClaims(req.RefreshToken, claims, func(token *jwt.Token) (interface{}, error) {
        return []byte(h.config.JWTSecret), nil
    })

    if err != nil || !token.Valid {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
        return
    }

    // Generate new tokens
    newToken, newRefreshToken, err := h.generateTokens(claims.Subject)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "token":         newToken,
        "refresh_token": newRefreshToken,
    })
}

func (h *AuthHandler) generateTokens(tenantID string) (string, string, error) {
    // Access token (15 minutes)
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
        Subject:   tenantID,
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
        IssuedAt:  jwt.NewNumericDate(time.Now()),
    })

    tokenString, err := token.SignedString([]byte(h.config.JWTSecret))
    if err != nil {
        return "", "", err
    }

    // Refresh token (7 days)
    refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
        Subject:   tenantID,
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
        IssuedAt:  jwt.NewNumericDate(time.Now()),
    })

    refreshTokenString, err := refreshToken.SignedString([]byte(h.config.JWTSecret))
    if err != nil {
        return "", "", err
    }

    return tokenString, refreshTokenString, nil
}

func generateAPIKey() string {
    return "dk_" + uuid.New().String()
}