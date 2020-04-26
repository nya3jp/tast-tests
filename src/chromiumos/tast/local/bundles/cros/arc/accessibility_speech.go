// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
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
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

// speechLog obtains the speech log of ChromeVox.
func speechLog(ctx context.Context, cvconn *chrome.Conn) ([]string, error) {
	// speechLog represents a log of accessibility speech.
	type speechLog struct {
		Text string `json:"textString_"`
		// Other values are not used in test.
	}
	var logs []speechLog
	if err := cvconn.Eval(ctx, "LogStore.instance.getLogsOfType(LogStore.LogType.SPEECH)", &logs); err != nil {
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
	testActivities := []accessibility.TestActivity{accessibility.MainActivity}
	testFunc := func(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn, currentActivity accessibility.TestActivity) error {
		const (
			nextKey     = "Search+Right"
			activateKey = "Search+Space"
		)

		// Enable speech logging.
		if err := cvconn.Exec(ctx, `ChromeVoxPrefs.instance.setLoggingPrefs(ChromeVoxPrefs.loggingPrefs.SPEECH, true)`); err != nil {
			return errors.Wrap(err, "could not enable speech logging")
		}
		ew, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Error with creating EventWriter from keyboard: ", err)
		}
		defer ew.Close()

		for _, testStep := range []struct {
			key      string
			wantLogs []string
		}{
			{
				nextKey,
				[]string{"OFF", "Toggle Button", "Not pressed", "Press Search+Space to toggle"},
			}, {
				nextKey,
				[]string{"CheckBox", "Check box", "Not checked", "Press Search+Space to toggle"},
			}, {
				nextKey,
				[]string{"seekBar", "Slider", "25", "Min 0", "Max 100"},
			}, {
				nextKey,
				[]string{"Slider", "3", "Min 0", "Max 10"},
			}, {
				nextKey,
				[]string{"ANNOUNCE", "Button", "Press Search+Space to activate"},
			}, {
				activateKey,
				[]string{"test announcement"},
			}, {
				nextKey,
				[]string{"CLICK TO SHOW TOAST", "Button", "Press Search+Space to activate"},
			}, {
				activateKey,
				[]string{"test toast"},
			},
		} {
			// Ensure that ChromeVox log is cleared before proceeding.
			if err := cvconn.Exec(ctx, "LogStore.instance.clearLog()"); err != nil {
				return errors.Wrap(err, "error with clearing ChromeVox log")
			}
			if err := ew.Accel(ctx, testStep.key); err != nil {
				return errors.Wrapf(err, "accel(%s) returned error", testStep.key)
			}

			diff := ""
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				diff = ""
				gotLogs, err := speechLog(ctx, cvconn)
				if err != nil {
					return testing.PollBreak(err)
				}

				if diff = cmp.Diff(testStep.wantLogs, gotLogs); diff != "" {
					return errors.New("speech log was not as expected")
				}
				return nil
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				if diff != "" {
					// Write diff to file, if diff is observed after polling.
					diffFileName := "accessibility_speech_diff.txt"
					diffFilePath := filepath.Join(s.OutDir(), diffFileName)
					if writeFileErr := ioutil.WriteFile(diffFilePath, []byte("(-want +got):\n"+diff), 0644); writeFileErr != nil {
						return errors.Wrapf(err, "failed to write diff to the file; and the previous error is %v", writeFileErr)
					}
					return errors.Wrapf(err, "dumped diff to %s", diffFileName)
				}
				return errors.Wrap(err, "failed to check speech log")
			}
		}
		return nil
	}
	accessibility.RunTest(ctx, s, testActivities, testFunc)
}
