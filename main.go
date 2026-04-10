package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/alexrastr/tg-term/bot"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {

	ctx := context.Background()

	// === КАНАЛЫ ===
	incoming := make(chan bot.Message, 100) // из Telegram в UI
	outgoing := make(chan string, 100)      // из UI в Telegram
	errors := make(chan error, 100)         // ошибки
	alarms := make(chan struct{}, 10)       // [НОВОЕ] сигнал тревоги

	// === TELEGRAM BOT ===
	// Бот должен слать в alarms когда получает /alarm.
	// Передаём канал alarms в StartTelegram (нужно добавить поддержку в bot.go).
	go bot.StartTelegram(ctx, incoming, outgoing, errors, alarms)

	app := tview.NewApplication()

	// флаг: активна ли сейчас тревога (для остановки beep-горутины)
	var alarmActive atomic.Bool

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

	// показать модальное окно тревоги и запустить beep
	showAlarmModal := func() {
		// Если тревога уже активна — не показываем второй раз
		if alarmActive.Swap(true) {
			return
		}

		// Запускаем beep в отдельной горутине — системный bell на репите
		go func() {
			for alarmActive.Load() {
				fmt.Print("\a") // ASCII BEL — системный звуковой сигнал
				time.Sleep(800 * time.Millisecond)
			}
		}()

		modal := tview.NewModal().
			SetText("Новое сообщение").
			AddButtons([]string{"OK (Enter)"}).
			SetDoneFunc(func(_ int, _ string) {
				// Остановить звук и убрать модалку
				alarmActive.Store(false)
				app.SetRoot(buildLayout(chatView, input), true).SetFocus(input)
			})

		modal.SetBackgroundColor(tcell.ColorDefault)
		modal.SetBorderColor(tcell.ColorRed)

		app.SetRoot(modal, true).SetFocus(modal)
	}

	// входящие из Telegram → UI
	go func() {
		for msg := range incoming {
			fmt.Print("\a") // сигнал для терминала при получении нового сообщения
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

	// слушаем канал alarms
	go func() {
		for range alarms {
			app.QueueUpdateDraw(func() {
				addMessage("System", "/alarm получен")
				showAlarmModal()
			})
		}
	}()

	// время в заголовке
	go func() {
		for {
			timeStr := time.Now().Format("15:04:05")
			app.QueueUpdateDraw(func() {
				chatView.SetTitle("[:b] " + timeStr + " ")
			})
			time.Sleep(time.Second)
		}
	}()

	// Обработка Enter в поле ввода
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

	// Глобальные горячие клавиши
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			app.Stop()
			return nil
		}
		return event
	})

	// Запуск
	layout := buildLayout(chatView, input)
	if err := app.SetRoot(layout, true).SetFocus(input).Run(); err != nil {
		panic(err)
	}
}

// buildLayout собирает основной layout (вынесено, чтобы переиспользовать после закрытия модалки)
func buildLayout(chatView *tview.TextView, input *tview.InputField) *tview.Flex {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(chatView, 0, 1, false).
		AddItem(input, 3, 0, true)
}
