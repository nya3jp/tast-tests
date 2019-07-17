// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

// Used to keep information for a key, identified by the array of possible suffixes.
type keyInformation struct {
	// Possible suffixes for the key, for example "_latency_max"
	suffixes []string
	// Unit name, for example "ms"
	unitName string
	// Performance dirrection, for example perf.BiggerIsBetter.
	direction perf.Direction
}

var keyInformations = []keyInformation{
	keyInformation{
		suffixes:  []string{"_latency_max", "_latency_median", "_latency_min", "_time_max", "_time_median", "_time_min"},
		unitName:  "ms",
		direction: perf.SmallerIsBetter,
	},
	keyInformation{
		suffixes:  []string{"_missed_frame_percents"},
		unitName:  "percents",
		direction: perf.SmallerIsBetter,
	},
	keyInformation{
		suffixes:  []string{"_blend_rate", "_fill_rate"},
		unitName:  "screens",
		direction: perf.BiggerIsBetter,
	},
	keyInformation{
		suffixes:  []string{"_control_count"},
		unitName:  "controls",
		direction: perf.BiggerIsBetter,
	},
	keyInformation{
		suffixes:  []string{"_device_calls"},
		unitName:  "calls",
		direction: perf.BiggerIsBetter,
	},
	keyInformation{
		suffixes:  []string{"_triangle_count"},
		unitName:  "calls",
		direction: perf.BiggerIsBetter,
	},
	keyInformation{
		suffixes:  []string{"_fps"},
		unitName:  "fps",
		direction: perf.BiggerIsBetter,
	},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GamePerformance,
		Desc:         "Captures set of performance metrics and upload it to the server",
		Contacts:     []string{"khmel@chromium.org", "skuhne@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{"ArcGamePerformanceTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      1 * time.Hour,
	})
}

func GamePerformance(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	const apkName = "ArcGamePerformanceTest.apk"
	s.Log("Installing: ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	s.Log("Running test")
	out, err := a.Command(ctx, "am", "instrument", "-w", "android.gameperformance").CombinedOutput()
	if err != nil {
		s.Fatal("Failed to execute test: ", err)
	}

	outputFile := filepath.Join(s.OutDir(), "test_log.txt")
	err = ioutil.WriteFile(outputFile, []byte(out), 0644)
	if err != nil {
		s.Fatal("Failed to save test output: ", err)
	}

	s.Log("Analyzing results")
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	// Regular expression to verify test finished successfully.
	var reOkTests = regexp.MustCompile(`OK \(\d+ tests?\)`)

	// Make sure test is completed successfully.
	if len(lines) == 0 || !reOkTests.MatchString(lines[len(lines)-1]) {
		s.Fatal("Test is not completed successfully, see: " + outputFile)
	}

	perfValues := perf.NewValues()

	// Regular expression to find find instrumentation results.
	// Output may be prepended by other chars, and order of elements is not defined.
	// Examples:
	// INSTRUMENTATION_STATUS: opengl_post_time_min=431.0
	// .INSTRUMENTATION_STATUS: no_extra_load_blend_rate=20.0
	// android.gameperformance.GamePerformanceTest:INSTRUMENTATION_STATUS: opengl_fps=58.7376277786
	reInstrumentation := regexp.MustCompile(`INSTRUMENTATION_STATUS: (.+?)=(\d+.?\d*)`)

	for _, line := range lines {
		m := reInstrumentation.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := m[1]
		value, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			s.Fatal("Failed to parse float: ", err)
		}
		info, err := getKeyInformation(key)
		if err != nil {
			s.Fatal("Failed to parse key: ", err)
		}
		perfValues.Append(perf.Metric{
			Name:      key,
			Unit:      info.unitName,
			Direction: info.direction,
			Multiple:  true,
		}, value)
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
}

// getKeyInformation analyzes the key name and returns information that says if value defined by
// this key is better when bigger and unit name. This returns an error if key could not be
// recognized.
func getKeyInformation(key string) (keyInformation, error) {
	for _, testKeyInformation := range keyInformations {
		for _, suffix := range testKeyInformation.suffixes {
			if !strings.HasSuffix(key, suffix) {
				continue
			}
			return testKeyInformation, nil
		}
	}

	var empty keyInformation
	return empty, errors.New("Key could not be recognized: " + key)
}
