// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AudioMultiRates,
		Desc:     "Tests different combinations of channel number and sample rate on the container's audio (through alsa) using a pre-built crostini image",
		Contacts: []string{"judyhsiao@chromium.org", "cros-containers-dev@google.com", "chromeos-audio-bugs@google.com"},
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

// AudioMultiRates tests multiple sample rate playback capability.
func AudioMultiRates(ctx context.Context, s *testing.State) {
	const (
		noStreamsTimeout  = 20 * time.Second
		hasStreamsTimeout = 10 * time.Second
	)

	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	rates := []uint32{8000, 11025, 16000, 22050, 32000, 44100, 48000}
	for _, rate := range rates {
		s.Logf("Playback with %d Hz", rate)

		s.Log("Wait for all streams to stop")
		if err := crastestclient.WaitForNoStream(ctx, noStreamsTimeout); err != nil {
			s.Fatal("timeout waiting all streams stopped")
		}

		// Starts a goroutine to poll the audio streams created by aplay.
		resCh := crastestclient.StartPollStreamWorker(ctx, hasStreamsTimeout)
		if err := cont.Command(ctx, "aplay", "-r", strconv.Itoa(int(rate)), "-D", "hw:0,0", "-c", "2", "-f", "S16_LE", "-d", "5", "/dev/zero").Run(testexec.DumpLogOnError); err != nil {
			s.Fatalf("Failed to playback with %d Hz: %v", rate, err)
		}

		// verifying poll stream result.
		res := <-resCh
		if res.Error != nil {
			s.Fatal("Failed to poll streams: ", res.Error)
		}
		if len(res.Streams) != 1 {
			s.Fatalf("Unexpected number of streams: got %d, expect 1", len(res.Streams))
		}
		// Verifies the channel number.
		if res.Streams[0].FrameRate != rate {
			s.Fatalf("Unexpected sample rate: got %d, want %d", res.Streams[0].FrameRate, rate)
		}
	}
}
