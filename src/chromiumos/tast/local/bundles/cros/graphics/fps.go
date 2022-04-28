// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"bufio"
	"context"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FPS,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measure frames per second and check it is close to expected fps",
		Contacts:     []string{"drinkcat@chromium.org", "chromeos-gfx@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "no_chrome_dcheck", "no_qemu"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeGraphics",
		Data:         []string{"fps.html"},
		Timeout:      5 * time.Minute,
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

// parseTrace parses trace file in tracePath.
func parseTrace(tracePath string, crtc int) ([]float64, error) {
	// Line format:
	// <proc> [000] d.h1 87154.652132: drm_vblank_event: crtc=0, seq=49720
	// TODO(b/172225622): Do we need to care about seq?
	re := regexp.MustCompile(`^.* ([0-9\.]+): drm_vblank_event: crtc=(\d+).*$`)

	trace, err := os.Open(tracePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open trace file")
	}
	defer trace.Close()

	var data []float64
	lastEvent := 0.0
	scanner := bufio.NewScanner(trace)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); matches != nil {
			matchedCrtc, err := strconv.Atoi(matches[2])
			if err != nil {
				return nil, errors.Wrap(err, "error converting crtc to int")
			}
			if matchedCrtc != crtc {
				continue
			}

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
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error reading trace file")
	}

	if len(data) == 0 {
		return nil, errors.New("no data in trace file")
	}

	return data, nil
}

func FPS(ctx context.Context, s *testing.State) {
	const (
		tracingPath = "/sys/kernel/debug/tracing"

		// Collect statistics for 5 seconds.
		collectTime = 5 * time.Second

		// Trim 10% outliers of measurements.
		trimPercent = 10
		// Accept up to 0.2 fps margin over the reference fps.
		margin = 0.2
		// Accept up to 0.2 fps standard deviation.
		maxStddev = 0.2
	)

	// Open web page with constantly changing content to defeat PSR.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	testURL := server.URL + "/fps.html"

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}

	conn, err := cr.NewConn(ctx, testURL)
	if err != nil {
		s.Fatalf("Failed to open %s: %v", testURL, err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		s.Fatal("Waiting load failed: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize the keyboard writer: ", err)
	}
	defer kb.Close()

	// Iterate over each display.
	for _, info := range infos {
		// Displays may support many modes at the same refresh rates (e.g. different
		// resolutions). Instead of testing all modes, only test one mode per unique refresh
		// rate.
		rates := make(map[float64]*display.DisplayMode)
		for _, mode := range info.Modes {
			rates[mode.RefreshRate] = mode
		}

		// Iterate over each refresh rate for the current display.
		for _, mode := range rates {
			if err := display.SetDisplayProperties(ctx, tconn, info.ID,
				display.DisplayProperties{DisplayMode: mode}); err != nil {
				s.Fatal("Failed to set display properties: ", err)
			}

			// Wait for display configuration to update.
			// 5 seconds is chosen arbitrarily and may need adjustment.
			const configurationDelay = 5 * time.Second
			if err := testing.Sleep(ctx, configurationDelay); err != nil {
				s.Fatal("Cannot sleep: ", err)
			}

			// Acknowledge the configuration change or it will automatically revert
			// after some time.
			if err := kb.Accel(ctx, "Enter"); err != nil {
				s.Fatal("Failed to send keyboard action to confirm display configuration: ",
					err)
			}

			// Clear trace file.
			tracePath := filepath.Join(tracingPath, "trace")
			if err := ioutil.WriteFile(tracePath, nil, 0644); err != nil {
				s.Fatal("Cannot clear trace buffer: ", err)
			}
			defer ioutil.WriteFile(tracePath, nil, 0644)

			// Enable vblank events.
			vblankPath := filepath.Join(tracingPath,
				"events/drm/drm_vblank_event/enable")
			if err := ioutil.WriteFile(vblankPath, []byte("1"), 0644); err != nil {
				s.Fatal("Cannot enable drm vblank event tracing: ", err)
			}
			defer ioutil.WriteFile(vblankPath, []byte("0"), 0644)

			// Collect trace data.
			tracingOnPath := filepath.Join(tracingPath, "tracing_on")
			if err := ioutil.WriteFile(tracingOnPath, []byte("1"), 0644); err != nil {
				s.Fatal("Cannot enable tracing: ", err)
			}
			defer ioutil.WriteFile(tracingOnPath, []byte("0"), 0644)

			s.Log("Collecting vblank event samples")
			if err := testing.Sleep(ctx, collectTime); err != nil {
				s.Fatal("Cannot sleep: ", err)
			}

			ioutil.WriteFile(tracingOnPath, []byte("0"), 0644)

			// Save trace file in output directory.
			outputPath := filepath.Join(s.OutDir(), "trace.txt")
			if err := fsutil.CopyFile(tracePath, outputPath); err != nil {
				s.Fatal("Failed to copy trace file: ", err)
			}

			crtcs, err := graphics.ModetestCrtcs(ctx)
			if err != nil {
				s.Fatal("Failed to read crtcs from modetest: ", err)
			}

			// Check trace data for each crtc at its respective refresh rate.
			for index, crtc := range crtcs {
				if crtc.Mode == nil {
					continue
				}

				targetFPS := crtc.Mode.Refresh
				if targetFPS <= 0 {
					continue
				}
				s.Logf("Checking crtc=%d at %fHz", index, targetFPS)

				// Parse trace file and compute statistics.
				fpsData, err := parseTrace(outputPath, index)
				if err != nil {
					s.Fatal("Cannot parse trace: ", err)
				}

				sort.Float64s(fpsData)
				mean, stddev := float64Stats(fpsData)
				s.Logf("%d total samples, mean: %f, stddev: %f (min/max %f/%f)",
					len(fpsData), mean, stddev, fpsData[0],
					fpsData[len(fpsData)-1])

				// Trim outliers on each side.
				trim := len(fpsData) * trimPercent / 100
				fpsData = fpsData[trim : len(fpsData)-trim]

				mean, stddev = float64Stats(fpsData)
				s.Logf("%d trimmed samples, mean: %f, stddev: %f (min/max %f/%f)",
					len(fpsData), mean, stddev, fpsData[0],
					fpsData[len(fpsData)-1])

				// Check results.
				if mean > targetFPS+margin || mean < targetFPS-margin {
					s.Fatalf("Mean FPS %f out of expected range %f +/- %f",
						mean, targetFPS, margin)
				}

				// TODO(b/172225622): re-enable stddev check if we can find
				// meaningful bounds.
				if stddev > maxStddev {
					s.Logf("FPS standard deviation %f too large (> %f)", stddev,
						maxStddev)
				}
			}
		}

		// Move the window to the next display.
		if err := kb.Accel(ctx, "Search+Alt+M"); err != nil {
			s.Fatal("Failed to send keybord action to move the active window between displays: ",
				err)
		}
	}
}
