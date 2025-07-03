package keycloak

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/leozw/uptime-guardian/internal/config"
)

type Client struct {
	config    config.KeycloakConfig
	publicKey *rsa.PublicKey
}

func NewClient(cfg config.KeycloakConfig) *Client {
	return &Client{
		config: cfg,
	}
}

func (c *Client) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	log.Printf("Validating token for Keycloak URL: %s, Realm: %s", c.config.URL, c.config.Realm)

	// Get public key if not cached
	if c.publicKey == nil {
		log.Println("Public key not cached, fetching from Keycloak...")
		if err := c.fetchPublicKey(); err != nil {
			return nil, fmt.Errorf("failed to fetch public key: %w", err)
		}
	}

	// Parse and validate token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return c.publicKey, nil
	})

	if err != nil {
		log.Printf("Failed to parse token: %v", err)
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	// Log successful validation
	log.Printf("Token validated successfully for user: %v, org: %v", claims["email"], claims["organization"])

	// Validate expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, fmt.Errorf("token expired")
		}
	}

	return claims, nil
}

func (c *Client) fetchPublicKey() error {
	url := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", c.config.URL, c.config.Realm)
	log.Printf("Fetching JWKS from: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			Kid string   `json:"kid"`
			Kty string   `json:"kty"`
			Alg string   `json:"alg"`
			Use string   `json:"use"`
			N   string   `json:"n"`
			E   string   `json:"e"`
			X5c []string `json:"x5c"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode jwks: %w", err)
	}

	log.Printf("Found %d keys in JWKS", len(jwks.Keys))

	if len(jwks.Keys) == 0 {
		return fmt.Errorf("no keys found in jwks")
	}

	// Use the first RSA key for signing
	for _, key := range jwks.Keys {
		log.Printf("Key: kid=%s, kty=%s, use=%s, alg=%s", key.Kid, key.Kty, key.Use, key.Alg)
		if key.Kty == "RSA" && key.Use == "sig" {
			publicKey, err := c.parseJWK(key.N, key.E)
			if err != nil {
				log.Printf("Failed to parse key %s: %v", key.Kid, err)
				continue
			}
			c.publicKey = publicKey
			log.Printf("Successfully loaded RSA public key with kid: %s", key.Kid)
			return nil
		}
	}

	return fmt.Errorf("no suitable RSA signing key found")
}

func (c *Client) parseJWK(n, e string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, fmt.Errorf("failed to decode n: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, fmt.Errorf("failed to decode e: %w", err)
	}

	nBig := new(big.Int).SetBytes(nBytes)
	eBig := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: nBig,
		E: int(eBig.Int64()),
	}, nil
}
