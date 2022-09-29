// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
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
		LacrosStatus: testing.LacrosVariantUnneeded,
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
		captureDuration   = 1 // second(s)
		lowSliderGainMin  = 30
		sliderStepValue   = 10
		sliderStepDiffs   = 5
		highSliderGainMin = lowSliderGainMin + sliderStepDiffs*sliderStepValue
		gainTolerance     = 10.0
	)

	// Set input enabled(unmuted) by UI quicksettings.
	if err := setInputEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set mic unmuted: ", err)
	}

	type RMSValue struct {
		sliderVal int
		rms       float64
	}
	var rmsValues []RMSValue

	// Use loopback path to play and record data with specified input gain levels.
	for _, gainMin := range []int{lowSliderGainMin, highSliderGainMin} {
		currVal, err := quicksettings.SliderValue(ctx, tconn, quicksettings.SliderTypeMicGain)
		if err != nil {
			s.Fatal("Failed initial value check for mic gain slider: ", err)
		}
		s.Logf("Initial mic gain slider value: %d", currVal)

		// quicksettings library only supports coarse slider value adjustment (+-10 per step). We first
		// adjust the initial slider value to be located within [lowSliderGainMin, lowSliderGainMin+10)
		// for low gain, then increase the value until within [highSliderGainMin, highSliderGainMin+10)
		// for high gain.
		if currVal < gainMin {
			steps := int(math.Ceil(float64(gainMin-currVal) / sliderStepValue))
			for i := 0; i < steps; i++ {
				increase, err := quicksettings.IncreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeMicGain)
				if err != nil {
					s.Fatal("Failed to increase mic gain slider: ", err)
				}
				if increase != currVal+sliderStepValue {
					s.Fatalf("Failed to increase mic gain slider value; initial: %d, increased: %d", currVal, increase)
				}
				currVal = increase
			}
		} else {
			steps := (currVal - gainMin) / sliderStepValue
			for i := 0; i < steps; i++ {
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

		output := audio.TestRawData{
			Path:          filepath.Join(s.OutDir(), fmt.Sprintf("cras_recorded_%d.raw", currVal)),
			BitsPerSample: 16,
			Channels:      1,
			Rate:          48000,
			Duration:      captureDuration,
		}

		rms, err := playAndCaptureToCalculateRMS(ctx, input, output)
		if err != nil {
			s.Fatal("Failed to playback and capture: ", err)
		}
		s.Logf("Mic gain slider value = %d; Signal RMS amplitude = %f", currVal, rms)
		rmsValues = append(rmsValues, RMSValue{currVal, rms})
	}

	// Calculate the expected linear gain between the representative gains in decibel for the low and high
	// slider value according to cras/README.dbus-api:
	//         linearly maps [0, 50] to range [-20dB, 0dB] and [50, 100] to [0dB, 20dB]
	lowGainDB := float64(rmsValues[0].sliderVal)*20.0/50.0 - 20.0
	highGainDB := float64(rmsValues[1].sliderVal-50) * 20.0 / 50.0
	expectedGainLinear := math.Pow(10.0, (highGainDB-lowGainDB)/20.0)
	s.Logf("Expected gain = %.4f", expectedGainLinear)

	gainLinear := rmsValues[1].rms / rmsValues[0].rms
	s.Logf("Calculated gain = %.4f", gainLinear)

	if math.Abs(gainLinear-expectedGainLinear) > gainTolerance {
		s.Errorf("Gain is beyond expectation: got %.4f, expected %.4f, tolerance %.4f", gainLinear, expectedGainLinear, gainTolerance)
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
	// TODO(b/187793602): use better slider automation controls if possible, instead of keyboard controls.
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
	if err := quicksettings.SelectAudioOption(shortCtx, tconn, "Loopback Playback"); err != nil {
		s.Fatal("Failed to select ALSA loopback output: ", err)
	}

	// After selecting Loopback Playback, SelectAudioOption() sometimes detected that audio setting
	// is still opened while it is actually fading out, and failed to select Loopback Capture.
	// Call Hide() and Show() to reset the quicksettings menu first.
	quicksettings.Hide(shortCtx, tconn)
	quicksettings.Show(shortCtx, tconn)

	if err := quicksettings.SelectAudioOption(shortCtx, tconn, "Loopback Capture"); err != nil {
		s.Fatal("Failed to select ALSA loopback input: ", err)
	}

	if testControl == gainSlider {
		testInputGain(shortCtx, s, tconn, kb, audioInput)
	} else { // muteButton
		testInputMute(shortCtx, s, tconn, audioInput)
	}
}
