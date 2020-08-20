// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AudioSanity,
		Desc:     "Runs a sanity test on the container's audio (through alsa) using a pre-built crostini image",
		Contacts: []string{"paulhsia@chromium.org", "cros-containers-dev@google.com", "chromeos-audio-bugs@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"keepState"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:              "artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniUnstable,
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "download_stretch",
			Pre:       crostini.StartedByDownloadStretch(),
			Timeout:   10 * time.Minute,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "download_buster",
			Pre:       crostini.StartedByDownloadBuster(),
			Timeout:   10 * time.Minute,
			ExtraAttr: []string{"informational"},
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func AudioSanity(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx, cont)
	s.Log("List alsa output devices")
	if err := cont.Command(ctx, "aplay", "-l").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to list alsa output devices: ", err)
	}

	alsaSinksPattern := regexp.MustCompile("1\talsa_output.hw_0_0\tmodule-alsa-sink.c\ts16le 2ch 48000Hz\t(IDLE|SUSPENDED)\n")
	if out, err := cont.Command(ctx, "pactl", "list", "sinks", "short").Output(); err != nil {
		s.Fatal("Failed to list pulseaudio sinks: ", err)
	} else if res := alsaSinksPattern.Match(out); !res {
		s.Fatal("Failed to load alsa device to pulseaudio:", string(out))
	}

	s.Log("Play zeros with alsa device")
	if err := cont.Command(ctx, "aplay", "-r", "48000", "-c", "2", "-d", "3", "-f", "dat", "/dev/zero").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to playback with alsa devices: ", err)
	}

	s.Log("List alsa input devices")
	if err := cont.Command(ctx, "arecord", "-l").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to list alsa input devices: ", err)
	}

	s.Log("Capture with alsa device")
	if err := cont.Command(ctx, "arecord", "-r", "48000", "-c", "2", "-d", "3", "-f", "dat", "/dev/null").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to capture with alsa devices: ", err)
	}
}
