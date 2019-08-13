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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenRotationPerf,
		Desc:         "Test ARC rotation performance",
		Contacts:     []string{"khmel@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		// Sunflower.apk taken from: https://github.com/googlesamples/android-sunflower
		// Commit hash: ce82cffeed8150cf97789065898f08f29a2a1c9b
		Data:    []string{"Sunflower.apk"},
		Pre:     arc.Booted(),
		Timeout: 8 * time.Minute,
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

	const apkName = "Sunflower.apk"
	s.Log("Installing ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	const (
		pkgName = "com.google.samples.apps.sunflower"
		actName = ".GardenActivity"
	)
	act, err := arc.NewActivity(a, pkgName, actName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Logf("Starting activity: %s/%s", pkgName, actName)
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start activity: ", err)
	}

	// Switch to the "Plant list" tab, which contains many widgets, they cover the entire screen,
	// and they relayout when the screen rotates.
	obj := d.Object(ui.ClassName("androidx.appcompat.app.ActionBar$Tab"), ui.Description("Plant list"))
	if err := obj.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to find 'Plant list' widget: ", err)
	}
	if err := obj.Click(ctx); err != nil {
		s.Fatal("Could not switch to 'Plant list' tab: ", err)
	}
	if err := act.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for idle: ", err)
	}

	// Leave Chromebook in reasonable state.
	rot0 := 0
	p := display.DisplayProperties{Rotation: &rot0}
	defer display.SetDisplayProperties(ctx, tconn, dispInfo.ID, p)

	samples, err := grabPerfSamples(ctx, tconn, a, act, dispInfo.ID)
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
	if err := values.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf sample file: ", err)
	}
}

// grabPerfSamples runs the performance test and returns the samples.
// The performance test consists of measuring how expensive, GFX-wise, is to rotate the device.
// The information is taken from "dumpsys gfxinfo".
func grabPerfSamples(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, act *arc.Activity, dispID string) (samples map[string][]float64, err error) {
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

	groups := []string{}
	numFrames := 0
	samples = make(map[string][]float64)

	const samplesPerRotation = 10
	for i := 0; i < samplesPerRotation; i++ {
		testing.ContextLog(ctx, "Iteration number: ", i)
		for _, rot := range []int{90, 180, 270, 0} {
			testing.ContextLog(ctx, "Rotating to: ", rot)

			// Samples are grouped by vertical/horizontal rotation.
			keySuffix := "-horizontal"
			if rot == 90 || rot == 270 {
				keySuffix = "-vertical"
			}

			if err := gfxinfoResetStats(ctx, a, act.PackageName()); err != nil {
				return nil, err
			}
			p := display.DisplayProperties{Rotation: &rot}
			if err := display.SetDisplayProperties(ctx, tconn, dispID, p); err != nil {
				return nil, errors.Wrapf(err, "failed to set rotation to %d", rot)
			}

			// TODO(crbug.com/1007397): Remove testing.Sleep once bug gets fixed.
			if err := testing.Sleep(ctx, 3*time.Second); err != nil {
				return nil, err
			}

			if err := act.WaitForIdle(ctx, 10*time.Second); err != nil {
				return nil, err
			}

			stats, err := gfxinfoDumpStats(ctx, a, act.PackageName())
			if err != nil {
				return nil, err
			}
			ss := string(stats)
			groups = re.FindStringSubmatch(ss)
			if len(groups) == 0 {
				testing.ContextLog(ctx, "Dumpsys output: ", ss)
				return nil, errors.New("failed to parse output")
			}
			const numFramesGroupIdx = 2
			if numFrames, err = strconv.Atoi(groups[numFramesGroupIdx]); err != nil {
				return nil, err
			} else if numFrames == 0 {
				return nil, errors.New("invalid number of frames: got 0; want > 0")
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
	// Returning dumpsys text output as it doesn't support Protobuf.
	output, err := a.Command(ctx, "dumpsys", "gfxinfo", pkgName).Output(testexec.DumpLogOnError)
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

// nameForMetric returns a sanitized name, one that can be used in perf.Metric.
func nameForMetric(key string) string {
	// perf/perf.go defines the list of valid chars for the metric name.
	return strings.Replace(key, " ", "_", -1)
}
