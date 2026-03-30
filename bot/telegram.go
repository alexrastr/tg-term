package bot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/joho/godotenv"
)

type Message struct {
	From string
	Text string
}

func loadToken() string {
	_ = godotenv.Load()
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		panic("BOT_TOKEN is not set in environment or .env file")
	}
	return token
}

func newProxyClient() *http.Client {
	proxyURL, err := url.Parse(os.Getenv("PROXY_URL"))
	if err != nil {
		return &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 30 * time.Second,
	}
}

func StartTelegram(ctx context.Context, incoming chan<- Message, outgoing <-chan string, errors chan<- error, alarms chan<- struct{}) {
	for {
		token := loadToken()
		ownerID, err := strconv.ParseInt(os.Getenv("OWNER_ID"), 10, 64)
		if err != nil {
			incoming <- Message{
				From: "System",
				Text: "Не задан OWNER_ID или он некорректный. Бот не будет работать.",
			}
			return
		}

		b, err := bot.New(token,
			bot.WithHTTPClient(30*time.Second, newProxyClient()),
		)
		if err != nil {
			fmt.Fprintln(os.Stderr, "debug:", err)
			return
		}

		var chatID int64

		// обработка входящих сообщений
		b.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypeContains, func(ctx context.Context, b *bot.Bot, update *models.Update) {
			chatID = update.Message.Chat.ID

			if chatID != ownerID {
				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   fmt.Sprintf("Извините, но я могу общаться только с владельцем бота. Ваш ID: %d.", chatID),
				})
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

		// отправка сообщений из UI
		go func() {
			for text := range outgoing {
				_, err := b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: ownerID,
					Text:   text,
				})

				if err != nil {
					errors <- fmt.Errorf("не отправлено: %s (%v)", text, err)
				}
			}
		}()

		// запуск бота (polling)
		b.Start(ctx)

		time.Sleep(5 * time.Second) // небольшая задержка перед перезапуском
	}
}
