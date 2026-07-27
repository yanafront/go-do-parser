package auth

import (
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidNickname    = errors.New("invalid nickname")
)

type Service struct {
	password string
	secret   []byte
	users    map[string]struct{}
}

type Claims struct {
	Nickname string `json:"nickname"`
	jwt.RegisteredClaims
}

func New(password, secret string, users []string) *Service {
	allowed := make(map[string]struct{}, len(users))
	for _, u := range users {
		n := normalizeNickname(u)
		if n != "" {
			allowed[n] = struct{}{}
		}
	}
	return &Service{
		password: password,
		secret:   []byte(secret),
		users:    allowed,
	}
}

func (s *Service) Login(password, nickname string) (string, string, error) {
	nick := normalizeNickname(nickname)
	if nick == "" {
		return "", "", ErrInvalidNickname
	}
	if len(s.users) > 0 {
		if _, ok := s.users[nick]; !ok {
			return "", "", ErrInvalidCredentials
		}
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(s.password)) != 1 {
		return "", "", ErrInvalidCredentials
	}
	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Nickname: nick,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   nick,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	})
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", "", err
	}
	return signed, nick, nil
}

func (s *Service) Validate(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	nick := normalizeNickname(claims.Nickname)
	if nick == "" {
		return nil, errors.New("invalid token")
	}
	claims.Nickname = nick
	return claims, nil
}

func normalizeNickname(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	s = strings.ToLower(s)
	return s
}
