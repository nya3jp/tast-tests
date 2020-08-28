// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrasInputGain,
		Desc:         "Tests that the input capture gain is controllable",
		Contacts:     []string{"johnylin@chromium.org", "cychiang@chromium.org"},
		SoftwareDeps: []string{"audio_play", "audio_record"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func CrasInputGain(ctx context.Context, s *testing.State) {
	const (
		cleanupTime     = 10 * time.Second
		captureDuration = 1 // second(s)
		lowGain         = 25
		highGain        = 75
		expectedGain    = 100
		gainTolerance   = 10
	)

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to connect to CRAS: ", err)
	}

	// Load ALSA loopback module.
	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		s.Fatal("Failed to load ALSA loopback module: ", err)
	}
	defer unload(ctx)

	// Generate sine raw input file lasts 5 seconds.
	audioInput := audio.TestRawData{
		Path:          filepath.Join(s.OutDir(), "5SEC.raw"),
		BitsPerSample: 16,
		Channels:      2,
		Rate:          48000,
		Frequencies:   []int{440, 440},
		Volume:        0.05,
		Duration:      5,
	}
	if err := audio.GenerateTestRawData(ctx, audioInput); err != nil {
		s.Fatal("Failed to generate audio test data: ", err)
	}
	defer os.Remove(audioInput.Path)

	// Reserve time to remove input file and unload ALSA loopback at the end of the test.
	shortCtx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	// Set ALSA loopback both input and output node as active nodes.
	loopbackNodes, err := cras.GetNodesByType(shortCtx, audio.AloopCrasNodeType)
	if err != nil {
		s.Fatal("Failed to get any loopback nodes: ", err)
	}
	if len(loopbackNodes) != 2 {
		var dir string
		if dir = "output"; loopbackNodes[0].IsInput {
			dir = "input"
		}
		s.Fatalf("Only find %s node for loopback, the other direction is missing", dir)
	}

	for _, n := range loopbackNodes {
		if err := cras.SetActiveNode(shortCtx, *n); err != nil {
			s.Fatal("Failed to set active node: ", err)
		}
	}

	// Use loopback path to play and record data with specified input gain levels.
	rmsValues := make(map[int]float64)
	for _, gain := range []int{lowGain, highGain} {
		s.Logf("Start testing loopback with gain %d", gain)

		audioOutput := audio.TestRawData{
			Path:          filepath.Join(s.OutDir(), fmt.Sprintf("cras_recorded_%d.raw", gain)),
			BitsPerSample: 16,
			Channels:      1,
			Rate:          48000,
			Duration:      captureDuration,
		}

		// TODO: Adjust input gain level to expected value by controlling the scroll in
		//       system tray. Do we need to enable "Modify mic gain in the system tray"
		//       flag manually?

		// Playback function by CRAS.
		playCmd := testexec.CommandContext(
			shortCtx, "cras_test_client",
			"--playback_file", audioInput.Path,
			"--num_channels", strconv.Itoa(audioInput.Channels),
			"--rate", strconv.Itoa(audioInput.Rate))
		playCmd.Start()

		// Wait a short time to make sure playback command is working.
		testing.Sleep(shortCtx, 1*time.Second)

		// Capture function by CRAS.
		captureErr := testexec.CommandContext(
			shortCtx, "cras_test_client",
			"--capture_file", audioOutput.Path,
			"--duration", strconv.Itoa(audioOutput.Duration),
			"--num_channels", strconv.Itoa(audioOutput.Channels),
			"--rate", strconv.Itoa(audioOutput.Rate),
			"--format", fmt.Sprintf("S%d_LE", audioOutput.BitsPerSample)).Run(testexec.DumpLogOnError)

		if err := playCmd.Wait(); err != nil {
			s.Fatal("Playback did not finish in time: ", err)
		}

		if captureErr != nil {
			s.Fatal("Failed to capture data: ", captureErr)
		}

		rms, err := audio.GetRmsAmplitude(shortCtx, audioOutput)
		if err != nil {
			s.Fatal("Failed to get RMS amplitude: ", err)
		}
		s.Logf("Signal RMS amplitude = %f", rms)
		rmsValues[gain] = rms
	}

	// Check the relative input gain.
	gain := rmsValues[highGain] / rmsValues[lowGain]
	if math.Abs(gain-expectedGain) > gainTolerance {
		s.Errorf("Gain is beyond expectation: got %.2f, expected %d, tolerance %d", gain, expectedGain, gainTolerance)
	}
}
