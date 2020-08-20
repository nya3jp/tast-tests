// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PulseAudioSanity,
		Desc:     "Runs a sanity test on the container's pusleaudio service using a pre-built crostini image",
		Contacts: []string{"paulhsia@chromium.org", "cros-containers-dev@google.com", "chromeos-audio-bugs@google.com"},
		Attr:     []string{"group:mainline"},
		Vars:     []string{"keepState"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniUnstable,
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "download_buster",
			Pre:       crostini.StartedByDownloadBuster(),
			Timeout:   10 * time.Minute,
			ExtraAttr: []string{"informational"},
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

// checkPulseDevices checks if ALSA device is in the list of sinks in pulseaudio by using command `pactl`.
func checkPulseDevices(ctx context.Context, s *testing.State, cont *vm.Container) {
	alsaSinksPattern := regexp.MustCompile("1\talsa_output.hw_0_0\tmodule-alsa-sink.c\ts16le 2ch 48000Hz\t(IDLE|SUSPENDED)\n")
	if out, err := cont.Command(ctx, "pactl", "list", "sinks", "short").Output(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to list pulseaudio sinks: ", err)
	} else if res := alsaSinksPattern.Match(out); !res {
		s.Fatal("Failed to load ALSA device to pulseaudio:", string(out))
	}
}

// controlPulse controls pulseaudio through systemctl with command `cmd`.
func controlPulse(ctx context.Context, s *testing.State, cont *vm.Container, cmd string) {
	s.Logf("%v pulseaudio service", cmd)
	// Use systemctl to control pulseaudio service.
	if err := cont.Command(ctx, "systemctl", " --user", cmd, "pulseaudio").Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Fail to %s pulseaudio: %v", cmd, err)
	}
}

func PulseAudioSanity(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx, cont)

	s.Log("List ALSA output devices")
	if err := cont.Command(ctx, "aplay", "-l").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to list ALSA output devices: ", err)
	}

	// Case 1: Stop pulseaudio and run playback to restart.
	controlPulse(ctx, s, cont, "stop")
	s.Log("Play zeros with ALSA device")
	if err := cont.Command(ctx, "aplay", "-f", "dat", "-d", " 3", "/dev/zero").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to playback with ALSA devices: ", err)
	}
	checkPulseDevices(ctx, s, cont)

	// Case 2: Restart pulseaudio.
	controlPulse(ctx, s, cont, "restart")
	checkPulseDevices(ctx, s, cont)

	// Case 3: Kill pulseaudio and run playback to restart.
	controlPulse(ctx, s, cont, "kill")
	s.Log("Play zeros with ALSA device")
	if err := cont.Command(ctx, "aplay", "-f", "dat", "-d", "3", "/dev/zero").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to playback with ALSA devices: ", err)
	}
	checkPulseDevices(ctx, s, cont)
}
