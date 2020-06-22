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
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

type axSpeechTestStep struct {
	Key      string
	WantLogs []string
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
	const axSpeechFilePrefix = "accessibility_speech"

	MainActivityTestSteps := []axSpeechTestStep{
		{
			"Search+Right",
			[]string{"OFF", "Toggle Button", "Not pressed", "Press Search+Space to toggle"},
		}, {
			"Search+Right",
			[]string{"CheckBox", "Check box", "Not checked", "Press Search+Space to toggle"},
		}, {
			"Search+Right",
			[]string{"seekBar", "Slider", "25", "Min 0", "Max 100"},
		}, {
			"Search+Right",
			[]string{"Slider", "3", "Min 0", "Max 10"},
		}, {
			"Search+Right",
			[]string{"ANNOUNCE", "Button", "Press Search+Space to activate"},
		}, {
			"Search+Space",
			[]string{"test announcement"},
		}, {
			"Search+Right",
			[]string{"CLICK TO SHOW TOAST", "Button", "Press Search+Space to activate"},
		}, {
			"Search+Space",
			[]string{"test toast"},
		},
	}
	testActivities := []accessibility.TestActivity{accessibility.MainActivity}
	speechTestSteps := make(map[string][]axSpeechTestStep)
	speechTestSteps[accessibility.MainActivity.Name] = MainActivityTestSteps
	testFunc := func(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn, currentActivity accessibility.TestActivity) error {
		// Enable speech logging.
		if err := cvconn.Eval(ctx, `ChromeVoxPrefs.instance.setLoggingPrefs(ChromeVoxPrefs.loggingPrefs.SPEECH, true)`, nil); err != nil {
			return errors.Wrap(err, "could not enable speech logging")
		}
		ew, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Error with creating EventWriter from keyboard: ", err)
		}
		defer ew.Close()

		testSteps := speechTestSteps[currentActivity.Name]
		for _, testStep := range testSteps {
			// Ensure that ChromeVox log is cleared before proceeding.
			if err := cvconn.Eval(ctx, "LogStore.instance.clearLog()", nil); err != nil {
				return errors.Wrap(err, "error with clearing ChromeVox log")
			}
			if err := ew.Accel(ctx, testStep.Key); err != nil {
				return errors.Wrapf(err, "accel(%s) returned error", testStep.Key)
			}

			diff := ""
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				diff = ""
				gotLogs, err := speechLog(ctx, cvconn)
				if err != nil {
					return testing.PollBreak(err)
				}

				if diff = cmp.Diff(testStep.WantLogs, gotLogs); diff != "" {
					return errors.New("speech log was not as expected")
				}
				return nil
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
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
