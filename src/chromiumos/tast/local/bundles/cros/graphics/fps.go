// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FPS,
		Desc:         "Measure frames per second and check it is close to 60 fps",
		Contacts:     []string{"drinkcat@chromium.org", "chromeos-gfx@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func float64Stats(data []float64) (float64, float64) {
	var sum float64
	var sum2 float64
	for _, x := range data {
		sum += x
		sum2 += x * x
	}
	n := float64(len(data))
	mean := sum / n
	stddev := math.Sqrt((sum2 / n) - (mean * mean))

	return mean, stddev
}

// parseTrace parses trace file in tracePath, and also copies the content to outputPath.
func parseTrace(tracePath string, outputPath string) ([]float64, error) {
	// Line format:
	// <proc> [000] d.h1 87154.652132: drm_vblank_event: crtc=0, seq=49720
	// TODO: Do we need to care about crtc 0 and seq?
	re := regexp.MustCompile(`^.* ([0-9\.]+): drm_vblank_event: .*$`)

	trace, err := os.Open(tracePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open trace file")
	}
	defer trace.Close()

	output, err := os.Create(outputPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open trace file")
	}
	defer trace.Close()

	var data []float64
	lastEvent := 0.0
	scanner := bufio.NewScanner(trace)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(output, line)
		if matches := re.FindStringSubmatch(line); matches != nil {
			event, err := strconv.ParseFloat(matches[1], 64)
			if err != nil {
				return nil, errors.Wrap(err, "error converting time to float")
			}
			if lastEvent != 0.0 {
				data = append(data, 1.0/(event-lastEvent))
			}
			lastEvent = event
		}
	}
	if scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error reading trace file")
	}

	return data, nil
}

func FPS(ctx context.Context, s *testing.State) {
	const tracingPath = "/sys/kernel/debug/tracing"
	const vblankPath = tracingPath + "/events/drm/drm_vblank_event/enable"
	const tracingOnPath = tracingPath + "/tracing_on"
	const tracePath = tracingPath + "/trace"

	// Collect statistics for 5 seconds
	const moveTime = 5 * time.Second
	// Move mouse pointer every 5ms (needs to be faster than 60fps).
	const moveInterval = 5 * time.Millisecond

	// Trim 1% outliers of measurements
	const trimPercent = 1
	// Target fps (always 60 fps for now)
	const targetFPS = 60.0
	// Accept up to 0.2 fps margin over the reference 60 fps
	const margin = 0.2
	// Accept up to 0.1 fps standard deviation
	const maxStddev = 0.1

	mouse, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal(err, "cannot initialize mouse")
	}
	defer mouse.Close()

	if err := ioutil.WriteFile(tracePath, []byte(""), 0200); err != nil {
		s.Fatal("Cannot clear trace buffer: ", err)
	}
	defer ioutil.WriteFile(tracePath, []byte(""), 0200)
	if err := ioutil.WriteFile(vblankPath, []byte("1"), 0200); err != nil {
		s.Fatal("Cannot enable drm vblank event tracing: ", err)
	}
	defer ioutil.WriteFile(vblankPath, []byte("0"), 0200)
	if err := ioutil.WriteFile(tracingOnPath, []byte("1"), 0200); err != nil {
		s.Fatal("Cannot enable tracing: ", err)
	}
	defer ioutil.WriteFile(tracingOnPath, []byte("0"), 0200)

	if err := mouse.MoveCursor(moveTime, moveInterval); err != nil {
		s.Fatal("Error when moving mouse cursor: ", err)
		return
	}
	ioutil.WriteFile(tracingOnPath, []byte("0"), 0200)

	fpsData, err := parseTrace(tracePath, filepath.Join(s.OutDir(), "trace.txt"))
	if err != nil {
		s.Fatal("Cannot parse trace: ", err)
	}

	sort.Float64s(fpsData)
	mean, stddev := float64Stats(fpsData)
	s.Logf("%d total samples, mean: %f, stddev: %f (min/max %f/%f)",
		len(fpsData), mean, stddev, fpsData[0], fpsData[len(fpsData)-1])

	// Trim outliers on each side
	trim := len(fpsData) * trimPercent / 100
	fpsData = fpsData[trim : len(fpsData)-trim]

	mean, stddev = float64Stats(fpsData)
	s.Logf("%d trimmed samples, mean: %f, stddev: %f (min/max %f/%f)",
		len(fpsData), mean, stddev, fpsData[0], fpsData[len(fpsData)-1])

	if mean > targetFPS+margin || mean < targetFPS-margin {
		s.Fatalf("Mean FPS %f out of expected range %f +/- %f", mean, targetFPS, margin)
	}

	if stddev > maxStddev {
		s.Fatalf("FPS standard deviation %f too large (> %f)", stddev, maxStddev)
	}
}
