//go:build !windows

package main

func newPCSpeakerNotifier() notifier {
	return terminalBellNotifier{}
}
