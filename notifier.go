package main

import "strings"

type notifier interface {
	Play()
}

type noopNotifier struct{}

func (noopNotifier) Play() {}

type terminalBellNotifier struct{}

func (terminalBellNotifier) Play() {
	print("\a")
}

func newNotifier(mode string) notifier {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "terminal", "bell":
		return terminalBellNotifier{}
	case "off", "mute", "silent", "none":
		return noopNotifier{}
	case "pc-speaker", "pcspeaker", "speaker":
		return newPCSpeakerNotifier()
	default:
		return terminalBellNotifier{}
	}
}
