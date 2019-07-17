// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GamePerformance,
		Desc:         "Captures set of performance metrics and upload it to the server",
		Contacts:     []string{"khmel@chromium.org", "skuhne@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
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

	s.Log("Running test: ")
	cmd := a.Command(ctx, "am", "instrument", "-w", "android.gameperformance")
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.Fatal("Failed to execute test: ", err)
	}

	s.Log("Analyzing results")
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	// Make sure test is completed successfully.
	re := regexp.MustCompile("OK \\([\\d]* test[s]?\\)")
	if len(lines) == 0 ||
		re.FindString(lines[len(lines)-1]) == "" {
		s.Fatal("Test is not completed successfully\n" + string(out))
	}

	perfValues := perf.NewValues()

	// Output may be prepended by other chars, and order of elements is not defined.
	// Examples:
	// INSTRUMENTATION_STATUS: opengl_post_time_min=431.0
	// .INSTRUMENTATION_STATUS: no_extra_load_blend_rate=20.0
	// android.gameperformance.GamePerformanceTest:INSTRUMENTATION_STATUS: opengl_fps=58.7376277786
	resultPrefix := "INSTRUMENTATION_STATUS: "

	for _, line := range lines {
		index := strings.Index(line, resultPrefix)
		if index >= 0 {
			sub := line[index+len(resultPrefix):]
			keyValue := strings.Split(sub, "=")
			if len(keyValue) != 2 {
				s.Fatal("Failed to parse: ", line)
			}
			key := keyValue[0]
			value, err := strconv.ParseFloat(keyValue[1], 64)
			if err != nil {
				s.Fatal("Failed to parse float: ", err)
			}
			direction, unitName, err := getKeyInformation(key)
			if err != nil {
				s.Fatal("Failed to parse key: ", err)
			}
			perfValues.Append(perf.Metric{
				Name:      key,
				Unit:      unitName,
				Direction: direction,
				Multiple:  true,
			}, value)
		}
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
}

// GetKeyInformation analyzes the key name and returns information that says if value defined by
// this key is better when bigger and unit name. This returns an error if key could not be
// recognized.
func getKeyInformation(key string) (perf.Direction, string, error) {
	if strings.HasSuffix(key, "_latency_max") ||
		strings.HasSuffix(key, "_latency_median") ||
		strings.HasSuffix(key, "_latency_min") ||
		strings.HasSuffix(key, "_time_max") ||
		strings.HasSuffix(key, "_time_median") ||
		strings.HasSuffix(key, "_time_min") {
		return perf.SmallerIsBetter, "ms", nil
	}

	if strings.HasSuffix(key, "_fps") {
		return perf.BiggerIsBetter, "fps", nil
	}

	if strings.HasSuffix(key, "_missed_frame_percents") {
		return perf.SmallerIsBetter, "percents", nil
	}

	if strings.HasSuffix(key, "_blend_rate") ||
		strings.HasSuffix(key, "_fill_rate") {
		return perf.BiggerIsBetter, "screens", nil
	}

	if strings.HasSuffix(key, "_control_count") {
		return perf.BiggerIsBetter, "controls", nil
	}

	if strings.HasSuffix(key, "_device_calls") {
		return perf.BiggerIsBetter, "calls", nil
	}

	if strings.HasSuffix(key, "_triangle_count") {
		return perf.BiggerIsBetter, "triangles", nil
	}

	return perf.SmallerIsBetter, "", errors.New("Key could not be recognized: " + key)
}
