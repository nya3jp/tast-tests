// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"

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
		SoftwareDeps: []string{"chrome"},
		Data:         []string{accessibility.ApkName},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

// speechLog obtains the speech log of ChromeVox.
func speechLog(ctx context.Context, chromeVoxConn *chrome.Conn) ([]string, error) {
	// speechLog represents a log of accessibility speech.
	type speechLog struct {
		Text string `json:"textString_"`
		// Other values are not used in test.
	}
	var logs []speechLog
	if err := chromeVoxConn.Eval(ctx, "LogStore.instance.getLogsOfType(LogStore.LogType.SPEECH)", &logs); err != nil {
		return nil, err
	}
	var gotLogs []string
	for _, log := range logs {
		// TODO (crbug/1053374):Investigate cause of empty string.
		if log.Text != "" {
			gotLogs = append(gotLogs, log.Text)
		}
	}
	return gotLogs, nil
}

func AccessibilitySpeech(ctx context.Context, s *testing.State) {
	accessibility.RunTest(ctx, s, func(ctx context.Context, a *arc.ARC, chromeVoxConn *chrome.Conn, ew *input.KeyboardEventWriter) error {
		// Array containing expected speech logs from ChromeVox.
		expectedSpeechLogs := [][]string{
			{"OFF", "Toggle Button", "Not pressed", "Press Search+Space to toggle"},
			{"CheckBox", "Check box", "Not checked", "Press Search+Space to toggle"},
			{"seekBar", "Slider", "25", "Min 0", "Max 100"},
			{"seekBarDiscrete", "Slider", "3", "Min 0", "Max 10"},
			{"ANNOUNCE", "Button", "Press Search+Space to activate"},
		}

		// Enable speech logging.
		if err := chromeVoxConn.Exec(ctx, `ChromeVoxPrefs.instance.setLoggingPrefs(ChromeVoxPrefs.loggingPrefs.SPEECH, true)`); err != nil {
			return errors.Wrap(err, "could not enable speech logging")
		}

		// Move focus to each of the UI elements, and check that ChromeVox log speaks as expected.
		for _, wantLogs := range expectedSpeechLogs {
			// Ensure that ChromeVox log is cleared before proceeding.
			if err := chromeVoxConn.Exec(ctx, "LogStore.instance.clearLog()"); err != nil {
				return errors.Wrap(err, "error with clearing ChromeVox log")
			}
			if err := ew.Accel(ctx, "Search+Right"); err != nil {
				return errors.Wrap(err, "accel(Tab) returned error")
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				gotLogs, err := speechLog(ctx, chromeVoxConn)
				if err != nil {
					return testing.PollBreak(err)
				}
				if diff := cmp.Diff(wantLogs, gotLogs); diff != "" {
					return errors.Errorf("speech log was not as expected, diff is %q", diff)
				}
				return nil
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				return errors.Wrap(err, "failed to check speech log")
			}
		}
		return nil
	})
}
