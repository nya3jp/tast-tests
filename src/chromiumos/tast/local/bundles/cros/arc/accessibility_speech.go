// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{accessibility.ApkName},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
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
		return errors.Errorf("speech log was not as expected, got: %q want: %q", gotLogString, wantLog)
	}

	// Ensure that ChromeVox log is cleared before proceeding.
	if err := chromeVoxConn.Exec(ctx, "LogStore.instance.clearLog()"); err != nil {
		return errors.Wrap(err, "error with clearing ChromeVox Log")
	}
	return nil
}

func AccessibilitySpeech(ctx context.Context, s *testing.State) {
	const (
		appName                     = "Accessibility Test App"
		seekBarInitialValue         = 25
		seekBarDiscreteInitialValue = 3
	)

	accessibility.RunTest(ctx, s, func(ctx context.Context, a *arc.ARC, chromeVoxConn *chrome.Conn, ew *input.KeyboardEventWriter) error {
		// Array containg expected speech logs from ChromeVox.
		expectedSpeechLogs := []string{
			"OFF Toggle Button Not pressed Press Search+Space to toggle",
			"CheckBox Check box Not checked Press Search+Space to toggle",
			"seekBar Slider 25 Min 0 Max 100",
			"seekBarDiscrete Slider 3 Min 0 Max 10",
			"ANNOUNCE Button Press Search+Space to activate",
		}
		if err := accessibility.WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
			s.Fatal("Could not check if ChromeVox is speaking: ", err)
		}

		// Enable speech logging.
		if err := chromeVoxConn.Exec(ctx, `CommandHandler.onCommand("enableLogging")`); err != nil {
			s.Fatal("Could not enable speech logging: ", err)
		}

		// Ensure that ChromeVox log is cleared before proceeding.
		if err := chromeVoxConn.Exec(ctx, "LogStore.instance.clearLog()"); err != nil {
			s.Fatal("Error with clearing ChromeVox Log")
		}

		// Move focus to each of the UI elements, and check that ChromeVox log speaks as expected.
		for _, expectedLog := range expectedSpeechLogs {
			if err := ew.Accel(ctx, "Tab"); err != nil {
				s.Fatal(err, "Accel(Tab) returned error")
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := checkSpeechLog(ctx, chromeVoxConn, expectedLog); err != nil {
					return testing.PollBreak(err)
				}
				return nil
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				s.Fatal(err, "Failed to check speech log")
			}
		}
		return nil
	})
}
