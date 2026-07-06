package auth

import (
	"crypto/subtle"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Service struct {
	password string
	secret   []byte
}

type Claims struct {
	jwt.RegisteredClaims
}

func New(password, secret string) *Service {
	return &Service{
		password: password,
		secret:   []byte(secret),
	}
}

func (s *Service) Login(password string) (string, error) {
	if subtle.ConstantTimeCompare([]byte(password), []byte(s.password)) != 1 {
		return "", ErrInvalidCredentials
	}
	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "admin",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	})
	return token.SignedString(s.secret)
}

func (s *Service) Validate(tokenString string) error {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		return s.secret, nil
	})
	if err != nil {
		return err
	}
	if !token.Valid {
		return errors.New("invalid token")
	}
	return nil
}
