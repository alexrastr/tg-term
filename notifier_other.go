//go:build !windows && !linux

package main

func newPCSpeakerNotifier() notifier {
	return terminalBellNotifier{}
}
