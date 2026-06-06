package telegram

import (
	"fmt"

	"github.com/gotd/td/tg"
)

func DescribeSentCode(s *tg.AuthSentCode) string {
	if s == nil {
		return "код запрошен"
	}
	var where string
	switch s.Type.(type) {
	case *tg.AuthSentCodeTypeApp:
		where = "в приложении Telegram (чат «Telegram», не SMS)"
	case *tg.AuthSentCodeTypeSMS:
		where = "по SMS на этот номер"
	case *tg.AuthSentCodeTypeCall:
		where = "голосовым звонком"
	case *tg.AuthSentCodeTypeFlashCall:
		where = "звонком (последние цифры номера = код)"
	case *tg.AuthSentCodeTypeMissedCall:
		where = "пропущенным звонком"
	case *tg.AuthSentCodeTypeEmailCode:
		where = "на email, привязанный к аккаунту"
	case *tg.AuthSentCodeTypeFragmentSMS:
		where = "через Fragment SMS"
	default:
		where = fmt.Sprintf("тип %T", s.Type)
	}
	msg := "Код отправлен " + where
	if s.Timeout > 0 {
		msg += fmt.Sprintf(". Подождите до %d сек", s.Timeout)
	}
	if next := describeNextCodeType(s.NextType); next != "" {
		msg += ". Если не придёт — можно запросить повтор: " + next
	}
	return msg
}

func describeNextCodeType(t tg.AuthCodeTypeClass) string {
	if t == nil {
		return ""
	}
	switch t.(type) {
	case *tg.AuthCodeTypeSMS:
		return "SMS"
	case *tg.AuthCodeTypeCall:
		return "звонок"
	case *tg.AuthCodeTypeFlashCall:
		return "flash call"
	default:
		return ""
	}
}
