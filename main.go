package main

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/alexrastr/tg-term/bot"
	"github.com/alexrastr/tg-term/storage"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	ctx := context.Background()

	store, err := storage.OpenMessageStore("data")
	if err != nil {
		log.Fatalf("failed to open message store: %v", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			log.Printf("failed to close message store: %v", closeErr)
		}
	}()

	incoming := make(chan bot.Message, 100)
	outgoing := make(chan string, 100)
	errors := make(chan error, 100)
	alarms := make(chan struct{}, 10)

	go bot.StartTelegram(ctx, incoming, outgoing, errors, alarms)

	app := tview.NewApplication()

	var alarmActive atomic.Bool

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

	clearHistory := func() error {
		if err := store.Clear(); err != nil {
			return err
		}

		chatView.SetText("")
		return nil
	}

	handleMessage := func(author, text string, fromTelegram bool) {
		if text == "/clear" {
			if err := clearHistory(); err != nil {
				addMessage("System", fmt.Sprintf("clear history: %v", err))
				return
			}

			addMessage("System", "История очищена")
			if fromTelegram {
				outgoing <- "История очищена."
			}
			return
		}

		sentAt := time.Now()
		addMessageAt(author, text, sentAt)
		if err := store.Save(storage.MessageRecord{
			Author:    author,
			Text:      text,
			Timestamp: sentAt,
		}); err != nil {
			addMessage("System", fmt.Sprintf("save message: %v", err))
		}
	}

	history, err := store.LoadAll()
	if err != nil {
		log.Printf("failed to load message history: %v", err)
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
				fmt.Print("\a")
				time.Sleep(800 * time.Millisecond)
			}
		}()

		modal := tview.NewModal().
			SetText("Новое сообщение").
			AddButtons([]string{"OK (Enter)"}).
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
				fmt.Print("\a")
				handleMessage(msg.From, msg.Text, true)
			})
		}
	}()

	go func() {
		for err := range errors {
			app.QueueUpdateDraw(func() {
				addMessage("System", err.Error())
			})
		}
	}()

	go func() {
		for range alarms {
			app.QueueUpdateDraw(func() {
				addMessage("System", "/alarm получен")
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
		handleMessage("Вы", text, false)
		if text == "/clear" {
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
