package telegram

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

type envAuthenticator struct {
	phone    string
	password string
	logHint  func(string)
}

func newEnvAuthenticator(phone, password string, logHint func(string)) envAuthenticator {
	return envAuthenticator{phone: phone, password: password, logHint: logHint}
}

func (e envAuthenticator) Phone(ctx context.Context) (string, error) {
	return e.phone, nil
}

func (e envAuthenticator) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	if v := strings.TrimSpace(os.Getenv("TG_AUTH_CODE")); v != "" {
		return v, nil
	}
	if !isInteractive() {
		if e.logHint != nil {
			e.logHint("Telegram sent login code to your phone. Add variable TG_AUTH_CODE in Railway, redeploy, then remove TG_AUTH_CODE after success.")
		}
		return "", fmt.Errorf("TG_AUTH_CODE is not set")
	}
	fmt.Print("Enter Telegram code: ")
	code, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(code), nil
}

func (e envAuthenticator) Password(ctx context.Context) (string, error) {
	if v := strings.TrimSpace(os.Getenv("TG_AUTH_PASSWORD")); v != "" {
		return v, nil
	}
	if e.password != "" {
		return e.password, nil
	}
	if !isInteractive() {
		return "", fmt.Errorf("TG_AUTH_PASSWORD is not set (2FA enabled on your Telegram account)")
	}
	fmt.Print("Enter Telegram 2FA password: ")
	pass, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(pass), nil
}

func (e envAuthenticator) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	return nil
}

func (e envAuthenticator) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign up not supported")
}

func isInteractive() bool {
	if os.Getenv("RAILWAY_ENVIRONMENT") != "" {
		return false
	}
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func sessionExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
