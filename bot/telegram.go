package bot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Message struct {
	From string
	Text string
}

type TranslateFunc func(key string) string

func reportBotError(errors chan<- error, t TranslateFunc) func(error) {
	return func(err error) {
		if err == nil {
			return
		}

		msg := err.Error()
		if strings.Contains(strings.ToLower(msg), "terminated by other getupdates request") {
			errors <- fmt.Errorf("%s", t("bot_stopped_other_process"))
			return
		}

		errors <- err
	}
}

func newProxyClient(proxyRaw string) (*http.Client, error) {
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

func StartTelegram(ctx context.Context, token string, proxy string, ownerID string, t TranslateFunc, incoming chan<- Message, outgoing <-chan string, errors chan<- error, alarms chan<- struct{}) {
	for {
		if token == "" {
			errors <- fmt.Errorf("%s", t("bot_token_not_set"))
			return
		}

		ownerID_int64, err := strconv.ParseInt(ownerID, 10, 64)
		if err != nil {
			errors <- fmt.Errorf("OWNER_ID is missing or invalid, bot will not start")
			return
		}

		client, err := newProxyClient(proxy)
		if err != nil {
			errors <- err
		}

		b, err := bot.New(
			token,
			bot.WithHTTPClient(30*time.Second, client),
			bot.WithErrorsHandler(reportBotError(errors, t)),
		)
		if err != nil {
			errors <- fmt.Errorf("failed to initialize telegram bot: %w", err)
			time.Sleep(5 * time.Second)
			continue
		}

		var chatID int64

		b.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypeContains, func(ctx context.Context, b *bot.Bot, update *models.Update) {
			chatID = update.Message.Chat.ID

			if chatID != ownerID_int64 {
				_, sendErr := b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   fmt.Sprintf("%s %d.", t("unauthorized_user"), chatID),
				})
				if sendErr != nil {
					errors <- fmt.Errorf("%s: %w", t("not_sent"), sendErr)
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

		runCtx, cancelRun := context.WithCancel(ctx)
		senderStopped := make(chan struct{})
		go func() {
			defer close(senderStopped)

			for {
				select {
				case <-runCtx.Done():
					return
				case text, ok := <-outgoing:
					if !ok {
						return
					}

					_, sendErr := b.SendMessage(runCtx, &bot.SendMessageParams{
						ChatID: ownerID_int64,
						Text:   text,
					})
					if sendErr != nil && runCtx.Err() == nil {
						errors <- fmt.Errorf("%s: %s (%v)", t("not_sent"), text, sendErr)
					}
				}
			}
		}()

		b.Start(ctx)
		cancelRun()
		<-senderStopped

		errors <- fmt.Errorf("%s", t("bot_stopped"))
		time.Sleep(5 * time.Second)
	}
}
