//go:build windows

package main

import "syscall"

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	beepProc = kernel32.NewProc("Beep")
)

type pcSpeakerNotifier struct{}

func newPCSpeakerNotifier() notifier {
	return pcSpeakerNotifier{}
}

func (pcSpeakerNotifier) Play() {
	const frequency = 1200
	const durationMs = 180

	r1, _, _ := beepProc.Call(uintptr(frequency), uintptr(durationMs))
	if r1 == 0 {
		terminalBellNotifier{}.Play()
	}
}
