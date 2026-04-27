package i18n

import (
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"github.com/BurntSushi/toml"
)

var localizer *i18n.Localizer

func Init(lang string) error {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	if _, err := bundle.LoadMessageFile("i18n/en.toml"); err != nil {
		return err
	}
	if _, err := bundle.LoadMessageFile("i18n/ru.toml"); err != nil {
		return err
	}

	if lang == "" {
		lang = language.English.String()
	}

	localizer = i18n.NewLocalizer(bundle, lang, language.English.String())
	return nil
}

func T(key string) string {
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID: key,
	})

	if err != nil {
		return key
	}

	return msg
}
