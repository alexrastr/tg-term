package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/alexrastr/tg-term/bot"
	"github.com/alexrastr/tg-term/i18n"
	"github.com/alexrastr/tg-term/storage"
	"github.com/joho/godotenv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Config struct {
	BotToken   string
	OwnerID    string
	ProxyURL   string
	AppLang    string
	AlertSound string
}

func loadConfig() *Config {
	_ = godotenv.Load()
	return &Config{
		BotToken:   os.Getenv("BOT_TOKEN"),
		OwnerID:    os.Getenv("OWNER_ID"),
		ProxyURL:   os.Getenv("PROXY_URL"),
		AppLang:    os.Getenv("APP_LANG"),
		AlertSound: os.Getenv("ALERT_SOUND"),
	}
}

func main() {
	ctx := context.Background()

	incoming := make(chan bot.Message, 100)
	outgoing := make(chan string, 100)
	errors := make(chan error, 100)
	alarms := make(chan struct{}, 10)

	store, err := storage.OpenMessageStore("data")
	if err != nil {
		log.Fatalf("failed to open message store: %v", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			log.Printf("failed to close message store: %v", closeErr)
		}
	}()

	app := tview.NewApplication()

	var alarmActive atomic.Bool

	config := loadConfig()
	notifier := newNotifier(config.AlertSound)

	i18n.Init(config.AppLang)
	go bot.StartTelegram(ctx, config.BotToken, config.ProxyURL, config.OwnerID, i18n.T, incoming, outgoing, errors, alarms)

	chatView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false).
		SetChangedFunc(func() {
			app.Draw()
		})

	chatView.
		SetTitleAlign(tview.AlignRight).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).SetTitle(" Chat ")

	input := tview.NewInputField().
		SetLabel(" > ").
		SetFieldWidth(0)

	input.
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true)

	addMessageAt := func(author, msg string, ts time.Time) {
		timestamp := ts.Format("15:04:05")
		if chatView.GetText(false) != "" {
			fmt.Fprint(chatView, "\n")
		}
		fmt.Fprintf(chatView, "[gray]%s [white]%s: %s", timestamp, author, msg)
	}

	addMessage := func(author, msg string) {
		addMessageAt(author, msg, time.Now())
	}

	saveMessage := func(author, msg string, ts time.Time) {
		if err := store.Save(storage.MessageRecord{
			Author:    author,
			Text:      msg,
			Timestamp: ts,
		}); err != nil {
			addMessage(i18n.T("system"), fmt.Sprintf("save message: %v", err))
		}
	}

	addAndSaveMessageAt := func(author, msg string, ts time.Time) {
		addMessageAt(author, msg, ts)
		saveMessage(author, msg, ts)
	}

	clearHistory := func() error {
		if err := store.Clear(); err != nil {
			return err
		}

		chatView.SetText("")
		return nil
	}

	handleMessage := func(author, text string, fromTelegram bool) bool {
		if text == "/clear" {
			if err := clearHistory(); err != nil {
				addMessage(i18n.T("system"), fmt.Sprintf("clear history: %v", err))
				return true
			}

			addMessage(i18n.T("system"), i18n.T("history_cleared"))
			if fromTelegram {
				outgoing <- i18n.T("history_cleared")
			}
			return true
		}

		sentAt := time.Now()
		addAndSaveMessageAt(author, text, sentAt)

		if result, handled, err := runScriptCommand(ctx, text); handled {
			outputAt := time.Now()
			response := result
			if err != nil {
				response = err.Error()
			}

			addAndSaveMessageAt(i18n.T("system"), response, outputAt)
			if fromTelegram {
				outgoing <- response
			}
			return true
		}

		return false
	}

	history, err := store.LoadAll()
	if err != nil {
		addMessage(i18n.T("system"), fmt.Sprintf("failed to load message history: %v", err))
	} else {
		for _, record := range history {
			addMessageAt(record.Author, record.Text, record.Timestamp)
		}
	}

	showAlarmModal := func() {
		if alarmActive.Swap(true) {
			return
		}

		go func() {
			for alarmActive.Load() {
				notifier.Play()
				time.Sleep(800 * time.Millisecond)
			}
		}()

		modal := tview.NewModal().
			SetText(i18n.T("new_message")).
			AddButtons([]string{"Enter"}).
			SetDoneFunc(func(_ int, _ string) {
				alarmActive.Store(false)
				app.SetRoot(buildLayout(chatView, input), true).SetFocus(input)
			})

		modal.SetBackgroundColor(tcell.ColorDefault)
		modal.SetBorderColor(tcell.ColorRed)

		app.SetRoot(modal, true).SetFocus(modal)
	}

	go func() {
		for msg := range incoming {
			app.QueueUpdateDraw(func() {
				notifier.Play()
				_ = handleMessage(msg.From, msg.Text, true)
			})
		}
	}()

	go func() {
		for err := range errors {
			app.QueueUpdateDraw(func() {
				addMessage(i18n.T("system"), err.Error())
			})
		}
	}()

	go func() {
		for range alarms {
			app.QueueUpdateDraw(func() {
				addMessage(i18n.T("system"), i18n.T("new_alarm"))
				showAlarmModal()
			})
		}
	}()

	go func() {
		for {
			timeStr := time.Now().Format("15:04:05")
			app.QueueUpdateDraw(func() {
				chatView.SetTitle("[:b] " + timeStr + " ")
			})
			time.Sleep(time.Second)
		}
	}()

	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}

		text := input.GetText()
		if text == "" {
			return
		}

		input.SetText("")
		if handled := handleMessage(i18n.T("you"), text, false); handled {
			return
		}

		outgoing <- text
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			app.Stop()
			return nil
		}
		return event
	})

	layout := buildLayout(chatView, input)
	if err := app.SetRoot(layout, true).SetFocus(input).Run(); err != nil {
		panic(err)
	}
}

func buildLayout(chatView *tview.TextView, input *tview.InputField) *tview.Flex {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(chatView, 0, 1, false).
		AddItem(input, 3, 0, true)
}
