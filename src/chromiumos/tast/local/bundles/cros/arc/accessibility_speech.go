// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilitySpeech,
		Desc:         "Checks ChromeVox reads Android elements as expected",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{"ArcAccessibilityTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

func AccessibilitySpeech(ctx context.Context, s *testing.State) {
	const (
		apkName = "ArcAccessibilityTest.apk"
	)
	cr, err := accessibility.NewChrome(ctx)
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
	defer cr.Close(ctx)

	a, err := accessibility.NewARC(ctx, s.OutDir())
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
	defer a.Close()

	if err := accessibility.InstallAndStartSampleApp(ctx, a, s.DataPath(apkName)); err != nil {
		s.Fatal("Setting up ARC environment with accessibility failed: ", err)
	}

	if err := accessibility.EnableSpokenFeedback(ctx, cr, a); err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}

	chromeVoxConn, err := accessibility.ChromeVoxExtConn(ctx, cr)
	if err != nil {
		s.Fatal("Creating connection to ChromeVox extension failed: ", err)
	}
	defer chromeVoxConn.Close()

	// Wait for ChromeVox to stop speaking before trying to obtain the speech log.
	if err := accessibility.WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
		s.Fatal("Could not wait for ChromeVox to stop speaking: ", err)
	}

	expectedSpeechLog := []struct {
		className   string // className that should have focus.
		expectedLog string // expectedLog from ChromeVox, when className is focused.
	}{
		{
			className:   accessibility.ToggleButton,
			expectedLog: "OFF Toggle Button Not pressed button tooltip Press Search+Space to toggle.",
		},
		{
			className:   accessibility.CheckBox,
			expectedLog: "CheckBox Check box Not checked checkbox tooltip Press Search+Space to toggle.",
		},
		{
			className:   accessibility.SeekBar,
			expectedLog: "seekBar Slider 25 Min 0 Max 100",
		},
		{
			className:   accessibility.SeekBar,
			expectedLog: "seekBarDiscrete Slider 3 Min 0 Max 10",
		},
		{
			className:   accessibility.SeekBar,
			expectedLog: "ANNOUNCE Button Press Search plus Space to activate.",
		},
		{
			className:   accessibility.EditText,
			expectedLog: "Enter some text here. Edit text Enter Text Here Double tap to start editing",
		},
	}

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("error with creating EventWriter from keyboard")
	}
	defer ew.Close()

	// Ensure that ChromeVox log is cleared before proceeding.
	if err := chromeVoxConn.Exec(ctx, "LogStore.instance.clearLog()"); err != nil {
		s.Fatal("error with clearing ChromeVox Log")
	}

	// Move focus to each of the UI elements, and check that ChromeVox log speaks as expected.
	for _, log := range expectedSpeechLog {
		if err := ew.Accel(ctx, "Tab"); err != nil {
			s.Fatal(err, "Accel(Tab) returned error")
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := checkSpeechLog(ctx, chromeVoxConn, log.expectedLog); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			s.Fatal(err, "failed to check speech log")
		}
	}

	// For each key typed, check that the ChromeVox log matches this.
	for _, key := range []string{"h", "e", "l", "l", "o"} {
		if err := ew.Type(ctx, key); err != nil {
			s.Fatal(ctx, "could not type:")
		}
		if err := checkSpeechLog(ctx, chromeVoxConn, key); err != nil {
			s.Fatal(err, "Error checking speech log")
		}
	}
}

func checkSpeechLog(ctx context.Context, chromeVoxConn *chrome.Conn, wantLog string) error {
	if err := accessibility.WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
		return errors.Wrap(err, "could not check if ChromeVox is speaking")
	}

	gotLogs, err := accessibility.SpeechLog(ctx, chromeVoxConn)
	if err != nil {
		return errors.Wrap(err, "could not get speech log")
	}

	if gotLogString := strings.Join(gotLogs, " "); gotLogString != wantLog {
		return errors.Errorf("Speech log was not as expected for. Got: %q, Want: %q", gotLogString, wantLog)
	}

	// Ensure that ChromeVox log is cleared before proceeding.
	if err := chromeVoxConn.Exec(ctx, "LogStore.instance.clearLog()"); err != nil {
		return errors.Wrap(err, "error with clearing ChromeVox Log")
	}
	return nil
}
