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
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenRotationPerf,
		Desc:         "Test ARC rotation performance",
		Contacts:     []string{"khmel@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ApiDemos.apk"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

func ScreenRotationPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get a Test API connection: ", err)
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for idle CPU: ", err)
	}

	const apkName = "ApiDemos.apk"
	s.Log("Installing ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	const pkgName = "com.example.android.apis"
	// Any activity that is in constant "motion" should be Ok, except OpenGL ones.
	// TODO(ricardoq): Investigate why OpenGL-based activities, like .graphics.SurfaceViewOverlay,
	// don't update the "number of frames" stat.
	act, err := arc.NewActivity(a, pkgName, ".graphics.AnimateDrawables")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting activity: ", pkgName)
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start ApiDemos activity: ", err)
	}

	// ApiDemos.apk is a PreN application. Needs to be put in maximized mode "manually".
	if err := act.SetWindowState(ctx, arc.WindowStateMaximized); err != nil {
		s.Fatal("Failed to set change window state: ", err)
	}

	// Wait for the "Application needs to restart to resize" dialog that appears on all Pre-N apks.
	if err := waitForRestartDialogAndRestart(ctx, act, d); err != nil {
		s.Fatal("Failed to restart Pre-N application: ", err)
	}

	if err := gfxinfoResetStats(ctx, a, act.PackageName()); err != nil {
		s.Fatal("Failed to reset gfxinfo stats: ", err)
	}

	// Leave Chromebook in reasonable state.
	rot0 := 0
	p := display.DisplayProperties{Rotation: &rot0}
	defer display.SetDisplayProperties(ctx, tconn, dispInfo.ID, p)

	samples, err := grabPerfSamples(ctx, tconn, a, act.PackageName(), dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to grab performance samples: ", err)
	}

	values := perf.NewValues()
	for k, v := range samples {
		values.Append(perf.Metric{
			Name:      nameForMetric(k),
			Unit:      unitForMetric(k),
			Direction: directionForMetric(k),
			Multiple:  true,
		}, v...)
	}
	values.Save(s.OutDir())
}

// grabPerfSamples returns
func grabPerfSamples(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, pkgName, dispID string) (samples map[string][]float64, err error) {
	re := regexp.MustCompile(
		`(Total frames rendered): (?P<num_frames>\d+)\s+` +
			`(Janky frames): (\d+) \((?:\d+\.\d+|-?nan)%\)\s+` +
			`(?:50th percentile: \d+ms\s+)?` +
			`(90th percentile): (\d+)ms\s+` +
			`(95th percentile): (\d+)ms\s+` +
			`(99th percentile): (\d+)ms\s+` +
			`(Number Missed Vsync): (\d+)\s+` +
			`(Number High input latency): (\d+)\s+` +
			`(Number Slow UI thread): (\d+)\s+` +
			`(Number Slow bitmap uploads): (\d+)\s+` +
			`(Number Slow issue draw commands): (\d+)\s+`)

	// Original cheets_ScreenRotation was capturing frames for 5 seconds. Instead of waiting
	// for a fixed amount of time (5 seconds * 10 * 4 is about 3 minutes), we wait until we
	// capture 'wantedFramesPerSample' frames. Since the activity has non-stopping moving parts,
	// it is guaranteed that the wanted amount of frames will be captured.
	const (
		wantedFramesPerSample = 50
		samplesPerRotation    = 10
	)
	groups := []string{}
	numFrames := 0
	samples = make(map[string][]float64)

	for i := 0; i < samplesPerRotation; i++ {
		testing.ContextLog(ctx, "Iteration number: ", i)
		for _, rot := range []int{90, 180, 270, 0} {
			testing.ContextLog(ctx, "Rotating to: ", rot)

			// Samples are grouped by according to the rotation.
			keySuffix := "-horizontal"
			if rot == 90 || rot == 270 {
				keySuffix = "-vertical"
			}

			p := display.DisplayProperties{Rotation: &rot}
			if err := display.SetDisplayProperties(ctx, tconn, dispID, p); err != nil {
				return nil, errors.Wrapf(err, "failed to set rotation to %d", rot)
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				stats, err := gfxinfoDumpStats(ctx, a, pkgName)
				if err != nil {
					return err
				}
				ss := string(stats)
				groups = re.FindStringSubmatch(ss)
				if len(groups) == 0 {
					testing.ContextLog(ctx, "Output: ", ss)
					return errors.New("failed to parse output")
				}
				const numFramesGroupIdx = 2
				if numFrames, err = strconv.Atoi(groups[numFramesGroupIdx]); err != nil {
					return errors.Wrap(err, "failed to convert string to int")
				} else if numFrames < wantedFramesPerSample {
					return errors.Errorf("unexpected number of frames: got %d; want >= %d", numFrames, wantedFramesPerSample)
				}

				// After a successful dump, reset stats
				return gfxinfoResetStats(ctx, a, pkgName)
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
				return nil, errors.Wrap(err, "failed to grab frames")
			}

			// Skip regexp group 0, which is the one that contains all the groups together.
			for j := 1; j < len(groups); j++ {
				key := groups[j]
				value, err := strconv.Atoi(groups[j+1])
				if err != nil {
					return nil, err
				}
				if key == "Janky frames" {
					// Since "Janky frames" is meaningless by itself, we track the "Janky percentage" instead.
					key = "Janky percentage"
					value = 100 * value / numFrames
				}
				key = key + keySuffix
				samples[key] = append(samples[key], float64(value))
				j++
			}
		}
	}
	return samples, nil
}

// gfxinfoDumpStats returns the graphics stats associated with a package name.
func gfxinfoDumpStats(ctx context.Context, a *arc.ARC, pkgName string) ([]byte, error) {
	cmd := a.Command(ctx, "dumpsys", "gfxinfo", pkgName)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch dumpsys")
	}
	return output, nil
}

// gfxinfoResetStats resets the graphics stats associated with a package name.
func gfxinfoResetStats(ctx context.Context, a *arc.ARC, pkgName string) error {
	return a.Command(ctx, "dumpsys", "gfxinfo", pkgName, "reset").Run()
}

// directionForMetric returns whether Higher or Lower is better for a given metric key.
func directionForMetric(key string) perf.Direction {
	if strings.HasPrefix(key, "Total frames rendered") {
		return perf.BiggerIsBetter
	}
	return perf.SmallerIsBetter
}

// unitForMetric returns the type of unit that should be used for a given metric key.
func unitForMetric(key string) string {
	if strings.Contains(key, "percentage") {
		return "percent"
	}
	if strings.Contains(key, "percentile") {
		return "ms"
	}
	return "count"
}

// nameForMetric returns a valid name to be used in perf.Metric
func nameForMetric(key string) string {
	// perf/perf.go defines the list of valid chars for the metric name.
	return strings.Replace(key, " ", "_", -1)
}

// waitForRestartDialogAndRestart waits for the "Application needs to restart to resize" dialog.
// This dialog appears on all Pre-N applications that tries to switch between maximized / restored window states.
// See: http://cs/pi-arc-dev/frameworks/base/core/java/com/android/internal/policy/DecorView.java
func waitForRestartDialogAndRestart(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	// TODO(ricardoq): Merge this function with "uiWaitForRestartDialogAndRestart" from arc/pip.go
	obj := d.Object(ui.ClassName("android.widget.Button"),
		ui.ID("android:id/button1"),
		ui.TextMatches("(?i)Restart"))
	if err := obj.WaitForExists(ctx, 10*time.Second); err != nil {
		return err
	}
	return obj.Click(ctx)
}
