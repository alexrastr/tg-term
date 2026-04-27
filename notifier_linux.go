//go:build linux

package main

import (
	"os"
	"os/exec"
	"strconv"

	"golang.org/x/sys/unix"
)

const (
	pcSpeakerFrequency = 1200
	pcSpeakerDuration  = 180
	linuxKDMKTONE      = 0x4B30
)

type linuxPCSpeakerNotifier struct {
	beepBin string
	file    *os.File
}

func newPCSpeakerNotifier() notifier {
	if beepBin, err := exec.LookPath("beep"); err == nil {
		return linuxPCSpeakerNotifier{beepBin: beepBin}
	}

	for _, candidate := range []string{"/dev/tty", "/dev/console", "/dev/tty0", "/dev/vc/0"} {
		file, err := os.OpenFile(candidate, os.O_WRONLY, 0)
		if err != nil {
			continue
		}

		return linuxPCSpeakerNotifier{file: file}
	}

	return terminalBellNotifier{}
}

func (n linuxPCSpeakerNotifier) Play() {
	if n.beepBin != "" {
		_ = exec.Command(n.beepBin,
			"-f", strconv.Itoa(pcSpeakerFrequency),
			"-l", strconv.Itoa(pcSpeakerDuration),
		).Run()
		return
	}

	if n.file != nil {
		tone := (pcSpeakerDuration << 16) | (1193180 / pcSpeakerFrequency)
		if err := unix.IoctlSetInt(int(n.file.Fd()), linuxKDMKTONE, tone); err == nil {
			return
		}
	}

	terminalBellNotifier{}.Play()
}
