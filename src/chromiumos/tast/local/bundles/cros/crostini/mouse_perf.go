// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	appletFile string = "mouse_perf.py"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MousePerf,
		Desc:         "Performance test for mouse responsiveness",
		Contacts:     []string{"hollingum@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact, appletFile},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func MousePerf(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	tconn := pre.TestAPIConn
	cont := pre.Container

	// Get access to the mouse and put it nearer the top-left corner.
	mouse, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal("Failed to get the mouse: ", err)
	}
	defer mouse.Close()
	// TODO(hollingum): put some absolute positioning in the API.
	mouse.Move(-1000, -1000)

	// In order to correct for variance in the receive times, we track the send times.
	var sendTimes []float64
	var sendTimestampMs int64 = -1

	doMouseMove := func(ctx context.Context) error {
		if err := crostini.MatchScreenshotDominantColor(ctx, cr, colorcmp.RGB(127, 0, 255), filepath.Join(s.OutDir(), "screenshot.png")); err != nil {
			return err
		}
		for i := 0; i < 400; i++ {
			// Record the timestamp of the send.
			curTimestampMs := int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)
			if sendTimestampMs != -1 {
				sendTimes = append(sendTimes, float64(curTimestampMs-sendTimestampMs))
			}
			sendTimestampMs = curTimestampMs
			// Send the event.
			if err := mouse.Move(2, 2); err != nil {
				return err
			}
			// Wait a fixed time before sending the next event.
			testing.Sleep(ctx, 1*time.Millisecond)
		}
		return nil
	}

	// Install dependencies.
	aptCmd := cont.Command(ctx, "sudo", "apt-get", "-y", "install", "python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0")
	if err := aptCmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to run %s: %v", shutil.EscapeSlice(aptCmd.Args), err)
	}

	// Launch the app.
	if err := cont.PushFile(ctx, s.DataPath(appletFile), "/home/testuser/"+appletFile); err != nil {
		s.Fatalf("Failed to push %v to container: %v", appletFile, err)
	}
	output, err := crostini.RunWindowedApp(ctx, tconn, cont, pre.Keyboard, 30*time.Second, doMouseMove, true, "mouse_perf", []string{"python3", appletFile})
	if err != nil {
		s.Fatal("Failed to run app: ", err)
	}

	// Process the output to generate this run's stats.
	//
	// For every motion event the applet receives, it prints x, y coords, and the time, on separate lines (in that order).
	var outputParsed []float64
	var distances []float64
	var times []float64
	for _, v := range strings.Split(output, "\n") {
		if v == "" {
			continue
		} else if p, err := strconv.ParseFloat(v, 64); err == nil {
			outputParsed = append(outputParsed, p)
		} else {
			s.Fatalf("Failed to parse %s as a float: %v", v, err)
		}
	}
	if len(outputParsed) < 3 {
		s.Fatalf("Failed to sample mouse movements, only received %v samples", len(outputParsed))
	}
	numDeltas := len(outputParsed)/3 - 1
	for i := 0; i < numDeltas; i++ {
		cur := i * 3
		next := (i + 1) * 3
		timeDelta := outputParsed[next+2] - outputParsed[cur+2]
		// This works out to x + y (a.k.a manhattan distance)
		distanceDelta := (outputParsed[next] - outputParsed[cur]) + (outputParsed[next+1] - outputParsed[cur+1])
		// TODO(hollingum): For some reason every alternate sample shows no change from the previous. Ignore them until we figure out why.
		if timeDelta == 0 && distanceDelta == 0 {
			continue
		}
		distances = append(distances, distanceDelta)
		times = append(times, timeDelta)
	}
	sendStats := processStats(sendTimes)
	distanceStats := processStats(distances)
	timeStats := processStats(times)

	// Record the stats for Crosbolt.
	s.Log("distances: ", distanceStats)
	s.Log("receive times: ", timeStats)
	s.Log("send times: ", sendStats)
	value := perf.NewValues()
	value.Set(perf.Metric{
		Name:      "crostini_mouse_perf",
		Variant:   "time_range",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, timeStats.UpperBound-timeStats.LowerBound)
	value.Set(perf.Metric{
		Name:      "crostini_mouse_perf",
		Variant:   "time_stdev",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, timeStats.StandardDeviation)
	value.Set(perf.Metric{
		Name:      "crostini_mouse_perf",
		Variant:   "send_range",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, sendStats.UpperBound-sendStats.LowerBound)
	value.Set(perf.Metric{
		Name:      "crostini_mouse_perf",
		Variant:   "send_stdev",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, sendStats.StandardDeviation)
	value.Set(perf.Metric{
		Name:      "crostini_mouse_perf",
		Variant:   "distance_range",
		Unit:      "px",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, distanceStats.UpperBound-distanceStats.LowerBound)
	value.Set(perf.Metric{
		Name:      "crostini_mouse_perf",
		Variant:   "distance_stdev",
		Unit:      "px",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, distanceStats.StandardDeviation)
	value.Save(s.OutDir())
}

type stats struct {
	UpperBound        float64
	LowerBound        float64
	Average           float64
	StandardDeviation float64
}

func processStats(samples []float64) stats {
	var lb = samples[0]
	var ub = samples[0]
	var sum, sumVar float64
	for _, sample := range samples {
		ub = math.Max(ub, sample)
		lb = math.Min(lb, sample)
		sum += sample
	}
	n := float64(len(samples))
	avg := sum / n
	for _, sample := range samples {
		sumVar += (sample - avg) * (sample - avg)
	}
	return stats{
		UpperBound:        ub,
		LowerBound:        lb,
		Average:           avg,
		StandardDeviation: math.Sqrt(sumVar / (n - 1)),
	}
}
