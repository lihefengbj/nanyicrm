package common

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
	UserID   uint64 `json:"userId"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"` // access token TTL in seconds
}

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
)

func GenerateTokenPair(signingKey string, accessTTL, refreshTTL time.Duration, userID uint64, username string) (*TokenPair, error) {
	access, err := signToken(signingKey, userID, username, accessTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := signToken(signingKey, userID, username, refreshTTL)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(accessTTL.Seconds()),
	}, nil
}

func signToken(signingKey string, userID uint64, username string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := TokenClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(signingKey))
}

func ParseToken(signingKey, tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return []byte(signingKey), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}
