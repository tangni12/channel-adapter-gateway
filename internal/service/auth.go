package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AuthService struct {
	secret []byte
}

type TokenClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Expires  int64  `json:"expires"`
}

func NewAuthService(secret string) *AuthService {
	return &AuthService{secret: []byte(secret)}
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	saltText := base64.RawURLEncoding.EncodeToString(salt)
	sum := passwordHash(saltText, password)
	return "sha256$" + saltText + "$" + sum, nil
}

func VerifyPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 3 || parts[0] != "sha256" {
		return false
	}
	return hmac.Equal([]byte(parts[2]), []byte(passwordHash(parts[1], password)))
}

func (s *AuthService) IssueToken(userID uint, username, role string) (string, error) {
	claims := TokenClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		Expires:  time.Now().Add(24 * time.Hour).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadText := base64.RawURLEncoding.EncodeToString(payload)
	signature := s.sign(payloadText)
	return payloadText + "." + signature, nil
}

func (s *AuthService) ParseToken(token string) (*TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid token")
	}
	if !hmac.Equal([]byte(s.sign(parts[0])), []byte(parts[1])) {
		return nil, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	if claims.Expires < time.Now().Unix() {
		return nil, errors.New("token expired")
	}
	return &claims, nil
}

func (s *AuthService) sign(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func passwordHash(salt, password string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func BearerToken(header string) (string, error) {
	if header == "" {
		return "", fmt.Errorf("missing Authorization header")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", fmt.Errorf("invalid Authorization header")
	}
	return strings.TrimSpace(parts[1]), nil
}
