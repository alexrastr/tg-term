package main

import (
	"context"
	"fmt"
	"time"

	"github.com/alexrastr/tg-term/bot"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {

	ctx := context.Background()

	// сообщение внутри приложения

	// === КАНАЛЫ ===
	incoming := make(chan bot.Message, 100) // из Telegram в UI
	outgoing := make(chan string, 100)      // из UI в Telegram
	errors := make(chan error, 100)         // ошибки

	// === TELEGRAM BOT ===
	go bot.StartTelegram(ctx, incoming, outgoing, errors)

	app := tview.NewApplication()

	// Окно чата (история сообщений)
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

	// Поле ввода
	input := tview.NewInputField().
		SetLabel(" > ").
		SetFieldWidth(0)

	input.
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true)

	// Функция добавления сообщения
	addMessage := func(author, msg string) {
		timestamp := time.Now().Format("15:04:05")
		if chatView.GetText(false) != "" {
			fmt.Fprint(chatView, "\n")
		}

		fmt.Fprintf(chatView, "[gray]%s [white]%s: %s",
			timestamp, author, msg)
	}

	// входящие из Telegram → UI
	go func() {
		for msg := range incoming {
			app.QueueUpdateDraw(func() {
				addMessage(msg.From, msg.Text)
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

	//время в заголовке
	go func() {
		for {
			timeStr := time.Now().Format("15:04:05")
			app.QueueUpdateDraw(func() {
				chatView.SetTitle("[:b] " + timeStr + " ")
			})
			time.Sleep(time.Second)
		}
	}()

	// Обработка Enter
	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := input.GetText()
			if text != "" {
				addMessage("Вы", text)
				input.SetText("")
				outgoing <- text
			}
		}
	})

	// Layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(chatView, 0, 1, false).
		AddItem(input, 3, 0, true)

	// Глобальные горячие клавиши
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			app.Stop()
			return nil
		}
		return event
	})

	// Запуск
	if err := app.SetRoot(layout, true).SetFocus(input).Run(); err != nil {
		panic(err)
	}
}
