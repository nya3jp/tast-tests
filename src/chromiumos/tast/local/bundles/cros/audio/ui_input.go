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
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testControl int

const (
	// gainSlider tests the control of microphone gain scroll on UI.
	gainSlider testControl = iota
	// muteButton tests the toggle of microphone enable/mute button on UI.
	muteButton
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UIInput,
		Desc:         "Tests that the input is controllable by UI API",
		Contacts:     []string{"johnylin@chromium.org", "cychiang@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Speaker(), hwdep.Microphone()),
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{
			{
				Name: "gain",
				Val:  gainSlider,
			},
			{
				Name: "mute",
				Val:  muteButton,
			},
		},
	})
}

func setInputEnabled(ctx context.Context, tconn *chrome.TestConn, enabled bool) error {
	if err := quicksettings.ToggleMic(ctx, tconn, enabled); err != nil {
		return errors.Wrap(err, "set failed")
	}

	// Check the input is really enabled(unmuted) or disabled(muted).
	currEnabled, err := quicksettings.MicEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "check setting failed")
	}
	if currEnabled != enabled {
		return errors.New("setting is not as expectation")
	}
	return nil
}

func playAndCaptureToCalculateRMS(ctx context.Context, input, output audio.TestRawData) (float64, error) {
	// Playback function by CRAS.
	playCmd := crastestclient.PlaybackFileCommand(
		ctx, input.Path,
		input.Duration,
		input.Channels,
		input.Rate)
	playCmd.Start()

	// Wait a short time to make sure playback command is working.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		if waitErr := playCmd.Wait(); waitErr != nil {
			return 0.0, errors.Wrap(waitErr, "playback did not finish in time")
		}
		return 0.0, errors.Wrap(err, "wait 1sec failed")
	}

	// Capture function by CRAS.
	captureErr := crastestclient.CaptureFileCommand(
		ctx, output.Path,
		output.Duration,
		output.Channels,
		output.Rate).Run(testexec.DumpLogOnError)

	if err := playCmd.Wait(); err != nil {
		return 0.0, errors.Wrap(err, "playback did not finish in time")
	}

	if captureErr != nil {
		return 0.0, errors.Wrap(captureErr, "capture data failed")
	}

	rms, err := audio.GetRmsAmplitude(ctx, output)
	if err != nil {
		return 0.0, errors.Wrap(err, "get RMS failed")
	}
	return rms, nil
}

func testInputGain(ctx context.Context, s *testing.State, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, input audio.TestRawData) {
	const (
		captureDuration = 1 // second(s)
		lowSliderGain   = 30
		sliderStepValue = 10
		sliderStepDiffs = 5
		highSliderGain  = lowSliderGain + sliderStepDiffs*sliderStepValue
		// lowSliderGain = 30% --> -16dB
		// highSliderGain = 80% --> +12dB
		// expectedGain = dB2Linear(28dB) = 25.1188
		expectedGain  = 25.1188
		gainTolerance = 10.0
	)

	// Set input enabled(unmuted) by UI quicksettings.
	if err := setInputEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set mic unmuted: ", err)
	}

	// Use loopback path to play and record data with specified input gain levels.
	rmsValues := make(map[int]float64)
	for _, gain := range []int{lowSliderGain, highSliderGain} {
		s.Logf("Start testing loopback with expected mic gain slider value: %d", gain)

		output := audio.TestRawData{
			Path:          filepath.Join(s.OutDir(), fmt.Sprintf("cras_recorded_%d.raw", gain)),
			BitsPerSample: 16,
			Channels:      1,
			Rate:          48000,
			Duration:      captureDuration,
		}

		currVal, err := quicksettings.SliderValue(ctx, tconn, quicksettings.SliderTypeMicGain)
		if err != nil {
			s.Fatal("Failed initial value check for mic gain slider: ", err)
		}
		s.Logf("Initial mic gain slider value: %d", currVal)

		// quicksettings library only supports coarse slider value adjustment (+-10 per step), we need
		// to check in advacne if the gain difference is adjustable (divisible by step). The initial
		// gain value should be 50 while the device "Loopback Capture" is created by audio.loadAloop().
		diff := int(math.Abs(float64(currVal - gain)))
		if diff%sliderStepValue != 0 {
			s.Fatalf("Failed to adjust gain slider from %d to %d (step = %d)", currVal, gain, sliderStepValue)
		}

		for i := 0; i < diff/sliderStepValue; i++ {
			if currVal < gain {
				increase, err := quicksettings.IncreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeMicGain)
				if err != nil {
					s.Fatal("Failed to increase mic gain slider: ", err)
				}
				if increase != currVal+sliderStepValue {
					s.Fatalf("Failed to increase mic gain slider value; initial: %d, increased: %d", currVal, increase)
				}
				currVal = increase
			} else {
				decrease, err := quicksettings.DecreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeMicGain)
				if err != nil {
					s.Fatal("Failed to decrease mic gain slider: ", err)
				}
				if decrease != currVal-sliderStepValue {
					s.Fatalf("Failed to decrease mic gain slider value; initial: %d, decreased: %d", currVal, decrease)
				}
				currVal = decrease
			}
		}

		rms, err := playAndCaptureToCalculateRMS(ctx, input, output)
		if err != nil {
			s.Fatal("Failed to playback and capture: ", err)
		}
		s.Logf("Signal RMS amplitude = %f", rms)
		rmsValues[gain] = rms
	}

	// Check the relative input gain.
	gain := rmsValues[highSliderGain] / rmsValues[lowSliderGain]
	s.Logf("Calculated gain = %.4f", gain)
	if math.Abs(gain-expectedGain) > gainTolerance {
		s.Errorf("Gain is beyond expectation: got %.4f, expected %.4f, tolerance %.4f", gain, expectedGain, gainTolerance)
	}
}

func testInputMute(ctx context.Context, s *testing.State, tconn *chrome.TestConn, input audio.TestRawData) {
	const (
		captureDuration = 1 // second(s)
		rmsTolerance    = 0.00001
	)

	// Set input disabled(muted) by UI quicksettings, and enabled after test.
	if err := setInputEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set mic muted: ", err)
	}
	defer setInputEnabled(ctx, tconn, true)

	// Use loopback path to play and record data with input while muted.
	s.Log("Start testing loopback with input while muted")

	output := audio.TestRawData{
		Path:          filepath.Join(s.OutDir(), "cras_recorded_mute.raw"),
		BitsPerSample: 16,
		Channels:      1,
		Rate:          48000,
		Duration:      captureDuration,
	}

	rms, err := playAndCaptureToCalculateRMS(ctx, input, output)
	if err != nil {
		s.Fatal("Failed to playback and capture: ", err)
	}
	s.Logf("Signal RMS amplitude = %f", rms)

	// Check the RMS value is small enough while muted.
	if rms > rmsTolerance {
		s.Errorf("RMS is too high for mute: got %f, tolerance %f", rms, rmsTolerance)
	}
}

func UIInput(ctx context.Context, s *testing.State) {
	const cleanupTime = 10 * time.Second

	// system-tray-mic-gain is enabled as default on R86+ images.
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Set up the keyboard, which is used to increment/decrement the slider.
	// TODO(crbug/1123231): use better slider automation controls if possible, instead of keyboard controls.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	// Defer this after deferring quicksettings.Hide to make sure quicksettings is still open when we
	// get the failure info.
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Load ALSA loopback module.
	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		s.Fatal("Failed to load ALSA loopback module: ", err)
	}
	defer unload(ctx)

	testControl := s.Param().(testControl)

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

	// Select ALSA loopback output and input nodes as active nodes by UI.
	if err := quicksettings.SelectAudioOption(shortCtx, tconn, kb, "Loopback Playback"); err != nil {
		s.Fatal("Failed to select ALSA loopback output: ", err)
	}
	if err := quicksettings.SelectAudioOption(shortCtx, tconn, kb, "Loopback Capture"); err != nil {
		s.Fatal("Failed to select ALSA loopback input: ", err)
	}

	if testControl == gainSlider {
		testInputGain(shortCtx, s, tconn, kb, audioInput)
	} else { // muteButton
		testInputMute(shortCtx, s, tconn, audioInput)
	}
}
