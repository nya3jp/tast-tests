// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gameperformance

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

// Used to keep information for a key, identified by the array of possible suffixes.
var keyInformations = []struct {
	// Possible suffixes for the key, for example "_latency_max"
	suffixes []string
	// Unit name, for example "ms"
	unitName string
	// Performance dirrection, for example perf.BiggerIsBetter.
	direction perf.Direction
}{{
	suffixes:  []string{"_latency_max", "_latency_median", "_latency_min", "_time_max", "_time_median", "_time_min"},
	unitName:  "ms",
	direction: perf.SmallerIsBetter,
}, {
	suffixes:  []string{"_missed_frame_percents"},
	unitName:  "percents",
	direction: perf.SmallerIsBetter,
}, {
	suffixes:  []string{"_blend_rate", "_fill_rate"},
	unitName:  "screens",
	direction: perf.BiggerIsBetter,
}, {
	suffixes:  []string{"_control_count"},
	unitName:  "controls",
	direction: perf.BiggerIsBetter,
}, {
	suffixes:  []string{"_device_calls"},
	unitName:  "calls",
	direction: perf.BiggerIsBetter,
}, {
	suffixes:  []string{"_triangle_count"},
	unitName:  "kilo-triangles",
	direction: perf.BiggerIsBetter,
}, {
	suffixes:  []string{"_fps"},
	unitName:  "fps",
	direction: perf.BiggerIsBetter,
},
}

// RunTest executes subset of tests in ArcGamePerformanceTest.apk determined by the test class name.
func RunTest(ctx context.Context, s *testing.State, className string) {
	a := s.PreValue().(arc.PreData).ARC

	const apkName = "ArcGamePerformanceTest.apk"
	s.Log("Installing: ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	s.Log("Running test")
	out, err := a.Command(ctx, "am", "instrument", "-w", "-e", "class", "android.gameperformance."+className, "android.gameperformance").CombinedOutput()
	if err != nil {
		s.Fatal("Failed to execute test: ", err)
	}

	outputFile := filepath.Join(s.OutDir(), "test_log.txt")
	err = ioutil.WriteFile(outputFile, []byte(out), 0644)
	if err != nil {
		s.Fatal("Failed to save test output: ", err)
	}

	s.Log("Analyzing results")

	// Make sure test is completed successfully.
	if !regexp.MustCompile(`\nOK \(\d+ tests?\)\n*$`).Match(out) {
		s.Fatal("Test is not completed successfully, see: " + outputFile)
	}

	perfValues := perf.NewValues()

	// Output may be prepended by other chars, and order of elements is not defined.
	// Examples:
	// INSTRUMENTATION_STATUS: opengl_post_time_min=431.0
	// .INSTRUMENTATION_STATUS: no_extra_load_blend_rate=20.0
	// android.gameperformance.GamePerformanceTest:INSTRUMENTATION_STATUS: opengl_fps=58.7376277786
	for _, m := range regexp.MustCompile(`INSTRUMENTATION_STATUS: (.+?)=(\d+.?\d*)`).FindAllStringSubmatch(string(out), -1) {
		key := m[1]
		value, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			s.Fatal("Failed to parse float: ", err)
		}
		info, err := makeMetricInfo(key)
		if err != nil {
			s.Fatal("Failed to parse key: ", err)
		}
		perfValues.Append(info, value)
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
}

// makeMetricInfo creates a metric descricption that can be supplied for reporting with the actual
// value. Returns an error in case key is not recognized.
func makeMetricInfo(key string) (perf.Metric, error) {
	for _, ki := range keyInformations {
		for _, suffix := range ki.suffixes {
			if !strings.HasSuffix(key, suffix) {
				continue
			}
			return perf.Metric{
				Name:      key,
				Unit:      ki.unitName,
				Direction: ki.direction,
				Multiple:  true,
			}, nil
		}
	}

	return perf.Metric{}, errors.New("Key could not be recognized: " + key)
}
