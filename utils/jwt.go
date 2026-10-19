package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ephemeralSecretOnce sync.Once
	ephemeralSecret     string

	ErrInvalidToken   = errors.New("invalid or malformed token")
	ErrExpiredToken   = errors.New("token has expired")
	ErrWrongTokenType = errors.New("invalid token type")
	ErrMissingSecret  = errors.New("jwt secret is missing")
	ErrRevokedToken   = errors.New("token has been revoked")

	tokenBlocklist = make(map[string]time.Time)
	blocklistMutex sync.RWMutex
)

// RevokeToken adds a token string to the blocklist so it cannot be used again
func RevokeToken(tokenString string) {
	if tokenString == "" {
		return
	}
	blocklistMutex.Lock()
	defer blocklistMutex.Unlock()

	tokenBlocklist[tokenString] = time.Now().Add(RefreshTokenDuration)

	// Clean up expired entries
	now := time.Now()
	for t, exp := range tokenBlocklist {
		if now.After(exp) {
			delete(tokenBlocklist, t)
		}
	}
}

// IsTokenRevoked checks if a token is in the revocation blocklist
func IsTokenRevoked(tokenString string) bool {
	blocklistMutex.RLock()
	defer blocklistMutex.RUnlock()
	expiry, exists := tokenBlocklist[tokenString]
	if !exists {
		return false
	}
	return time.Now().Before(expiry)
}

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"

	AccessTokenDuration  = 1 * time.Hour
	RefreshTokenDuration = 7 * 24 * time.Hour
)

type TokenClaims struct {
	UserID    uint64   `json:"user_id"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles,omitempty"`
	TokenType string   `json:"token_type"` // "access" or "refresh"
	DeviceID  string   `json:"device_id,omitempty"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"` // in seconds for access token
}

func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET_KEY")
	if secret != "" {
		return []byte(secret)
	}

	ephemeralSecretOnce.Do(func() {
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err == nil {
			ephemeralSecret = hex.EncodeToString(bytes)
			Logger.Warn("JWT_SECRET_KEY not provided in environment. Generated ephemeral runtime secret.")
		} else {
			ephemeralSecret = "studio_crm_default_fallback_jwt_secret_key"
		}
	})

	return []byte(ephemeralSecret)
}

// GenerateTokenPair generates both an access token and a refresh token.
// deviceID binds the pair to the device it was issued for; pass "" when unknown.
func GenerateTokenPair(userID uint64, email string, roles []string, deviceID string) (*TokenPair, error) {
	secret := getJWTSecret()
	now := time.Now()

	// 1. Generate Access Token
	accessExpiry := now.Add(AccessTokenDuration)
	accessClaims := TokenClaims{
		UserID:    userID,
		Email:     email,
		Roles:     roles,
		TokenType: TokenTypeAccess,
		DeviceID:  deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			Issuer:    "studio_backend",
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	signedAccessToken, err := accessToken.SignedString(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// 2. Generate Refresh Token
	refreshExpiry := now.Add(RefreshTokenDuration)
	refreshClaims := TokenClaims{
		UserID:    userID,
		Email:     email,
		TokenType: TokenTypeRefresh,
		DeviceID:  deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(refreshExpiry),
			Issuer:    "studio_backend",
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	signedRefreshToken, err := refreshToken.SignedString(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  signedAccessToken,
		RefreshToken: signedRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(AccessTokenDuration.Seconds()),
	}, nil
}

// ValidateToken verifies token validity, signature, and expected token type
func ValidateToken(tokenString string, expectedTokenType string) (*TokenClaims, error) {
	if IsTokenRevoked(tokenString) {
		return nil, ErrRevokedToken
	}

	secret := getJWTSecret()

	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Strict algorithm verification
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if expectedTokenType != "" && claims.TokenType != expectedTokenType {
		return nil, ErrWrongTokenType
	}

	return claims, nil
}
