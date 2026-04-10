package bot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/joho/godotenv"
)

type Message struct {
	From string
	Text string
}

func reportBotError(errors chan<- error) func(error) {
	return func(err error) {
		if err == nil {
			return
		}

		msg := err.Error()
		if strings.Contains(strings.ToLower(msg), "terminated by other getupdates request") {
			errors <- fmt.Errorf("бот остановлен: этот токен уже используется в другом процессе")
			return
		}

		errors <- err
	}
}

func loadToken() (string, error) {
	_ = godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return "", fmt.Errorf("BOT_TOKEN is not set in environment or .env file")
	}

	return token, nil
}

func newProxyClient() (*http.Client, error) {
	proxyRaw := os.Getenv("PROXY_URL")
	if proxyRaw == "" {
		return &http.Client{
			Timeout: 30 * time.Second,
		}, nil
	}

	proxyURL, err := url.Parse(proxyRaw)
	if err != nil {
		return &http.Client{
			Timeout: 30 * time.Second,
		}, fmt.Errorf("invalid PROXY_URL: %w", err)
	}

	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 30 * time.Second,
	}, nil
}

func StartTelegram(ctx context.Context, incoming chan<- Message, outgoing <-chan string, errors chan<- error, alarms chan<- struct{}) {
	for {
		token, err := loadToken()
		if err != nil {
			errors <- err
			return
		}

		ownerID, err := strconv.ParseInt(os.Getenv("OWNER_ID"), 10, 64)
		if err != nil {
			errors <- fmt.Errorf("OWNER_ID is missing or invalid, bot will not start")
			return
		}

		client, err := newProxyClient()
		if err != nil {
			errors <- err
		}

		b, err := bot.New(
			token,
			bot.WithHTTPClient(30*time.Second, client),
			bot.WithErrorsHandler(reportBotError(errors)),
		)
		if err != nil {
			errors <- fmt.Errorf("failed to initialize telegram bot: %w", err)
			time.Sleep(5 * time.Second)
			continue
		}

		var chatID int64

		b.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypeContains, func(ctx context.Context, b *bot.Bot, update *models.Update) {
			chatID = update.Message.Chat.ID

			if chatID != ownerID {
				_, sendErr := b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   fmt.Sprintf("Извините, но я могу общаться только с владельцем бота. Ваш ID: %d.", chatID),
				})
				if sendErr != nil {
					errors <- fmt.Errorf("failed to send unauthorized-user message: %w", sendErr)
				}
				return
			}

			if update.Message.Text == "/alarm" {
				alarms <- struct{}{}
				return
			}

			incoming <- Message{
				From: update.Message.From.Username,
				Text: update.Message.Text,
			}
		})

		go func() {
			for text := range outgoing {
				_, sendErr := b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: ownerID,
					Text:   text,
				})
				if sendErr != nil {
					errors <- fmt.Errorf("не отправлено: %s (%v)", text, sendErr)
				}
			}
		}()

		b.Start(ctx)

		errors <- fmt.Errorf("telegram bot stopped, restarting in 5 seconds")
		time.Sleep(5 * time.Second)
	}
}
