package telegram

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-faster/errors"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

func Login(ctx context.Context, apiID int, apiHash, phone, dataDir string) error {
	phone = NormalizePhone(phone)
	if phone == "" {
		return fmt.Errorf("phone is empty")
	}

	sessionPath, err := prepareSession(dataDir)
	if err != nil {
		return err
	}

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: sessionPath},
	})

	return client.Run(ctx, func(ctx context.Context) error {
		authClient := client.Auth()
		status, err := authClient.Status(ctx)
		if err != nil {
			return err
		}
		if status.Authorized {
			fmt.Println("Уже авторизовано, session.json актуален")
			return nil
		}

		fmt.Printf("Номер: %s\n", MaskPhone(phone))
		fmt.Println("Запрашиваю код у Telegram...")

		sentRaw, err := authClient.SendCode(ctx, phone, auth.SendCodeOptions{})
		if err != nil {
			if wait, ok := tgerr.AsFloodWait(err); ok {
				return fmt.Errorf("слишком много попыток, подождите %v: %w", wait, err)
			}
			return fmt.Errorf("send code: %w", err)
		}

		sent, ok := sentRaw.(*tg.AuthSentCode)
		if !ok {
			return fmt.Errorf("unexpected sent code type: %T", sentRaw)
		}

		fmt.Println(DescribeSentCode(sent))

		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("Код из Telegram (s = отправить снова, q = выход): ")
			line, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if line == "q" || line == "Q" {
				return fmt.Errorf("отменено")
			}
			if line == "s" || line == "S" {
				fmt.Println("Повторная отправка...")
				resentRaw, err := client.API().AuthResendCode(ctx, &tg.AuthResendCodeRequest{
					PhoneNumber:   phone,
					PhoneCodeHash: sent.PhoneCodeHash,
				})
				if err != nil {
					if wait, ok := tgerr.AsFloodWait(err); ok {
						fmt.Printf("Подождите %v перед повтором\n", wait)
						continue
					}
					return fmt.Errorf("resend code: %w", err)
				}
				resent, ok := resentRaw.(*tg.AuthSentCode)
				if !ok {
					return fmt.Errorf("unexpected resent code type: %T", resentRaw)
				}
				sent = resent
				fmt.Println(DescribeSentCode(sent))
				continue
			}

			_, signInErr := authClient.SignIn(ctx, phone, line, sent.PhoneCodeHash)
			if errors.Is(signInErr, auth.ErrPasswordAuthNeeded) {
				fmt.Print("Пароль 2FA: ")
				passLine, err := reader.ReadString('\n')
				if err != nil {
					return err
				}
				if _, err := authClient.Password(ctx, strings.TrimSpace(passLine)); err != nil {
					return fmt.Errorf("2FA: %w", err)
				}
				return nil
			}
			if signInErr != nil {
				fmt.Printf("Неверный код: %v\n", signInErr)
				continue
			}
			return nil
		}
	})
}
