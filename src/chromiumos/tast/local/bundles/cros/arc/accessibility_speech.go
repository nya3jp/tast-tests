// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
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
		Data:         []string{"accessibility_speech.MainActivity.json"},
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

// getSpeechTestSteps returns a slice of axSpeechTestStep, which is read from the specific file.
// TODO(b/155949540): Migrate json to golang struct, so this parsing will no longer be necessary.
func getSpeechTestSteps(filepath string) ([]axSpeechTestStep, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var steps []axSpeechTestStep
	err = json.NewDecoder(f).Decode(&steps)
	return steps, err
}

func AccessibilitySpeech(ctx context.Context, s *testing.State) {
	const axSpeechFilePrefix = "accessibility_speech"

	testActivities := []accessibility.TestActivity{accessibility.MainActivity}
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

		axSpeechTestSteps, err := getSpeechTestSteps(s.DataPath(axSpeechFilePrefix + currentActivity.Name + ".json"))
		if err != nil {
			return errors.Wrap(err, "error reading from JSON")
		}
		for _, axSpeechTestStep := range axSpeechTestSteps {
			// Ensure that ChromeVox log is cleared before proceeding.
			if err := cvconn.Eval(ctx, "LogStore.instance.clearLog()", nil); err != nil {
				return errors.Wrap(err, "error with clearing ChromeVox log")
			}
			if err := ew.Accel(ctx, axSpeechTestStep.Key); err != nil {
				return errors.Wrapf(err, "accel(%s) returned error", axSpeechTestStep.Key)
			}

			diff := ""
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				diff = ""
				gotLogs, err := speechLog(ctx, cvconn)
				if err != nil {
					return testing.PollBreak(err)
				}

				if diff = cmp.Diff(axSpeechTestStep.WantLogs, gotLogs); diff != "" {
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
