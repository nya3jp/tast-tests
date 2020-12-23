// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/screen"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ScreenRotationPerf,
		Desc: "Test ARC rotation performance",
		Contacts: []string{
			"khmel@chromium.org", // Maintainer.
			"arc-framework+tast@google.com",
			"ricardoq@chromium.org", // Tast port author.
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		// Sunflower.apk taken from: https://github.com/googlesamples/android-sunflower
		// Commit hash: ce82cffeed8150cf97789065898f08f29a2a1c9b
		Data:    []string{"Sunflower.apk"},
		Timeout: 8 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ScreenRotationPerf(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get a Test API connection: ", err)
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
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
	if err := act.Start(ctx, tconn); err != nil {
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

	// Leave Chromebook in reasonable state.
	rot0 := 0
	p := display.DisplayProperties{Rotation: &rot0}
	defer display.SetDisplayProperties(ctx, tconn, dispInfo.ID, p)

	// And right before starting the perf test, wait for an idle CPU.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for idle CPU: ", err)
	}

	samples, err := grabPerfSamples(ctx, tconn, a, d, pkgName, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to grab performance samples: ", err)
	}

	values := perf.NewValues()
	for pm, v := range samples {
		values.Append(pm, v...)
		accum := 0.0
		for _, n := range v {
			accum += n
		}
		s.Logf("Average: %q = %v", pm.Name, accum/float64(len(v)))
	}
	if err := values.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf sample file: ", err)
	}
}

// grabPerfSamples runs the performance test and returns the samples.
// The performance test consists of measuring how expensive, GFX-wise, is to rotate the device.
// The information is taken from "dumpsys gfxinfo".
func grabPerfSamples(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, pkgName, dispID string) (samples map[perf.Metric][]float64, err error) {
	samples = make(map[perf.Metric][]float64)
	const samplesPerRotation = 10
	for i := 0; i < samplesPerRotation; i++ {
		testing.ContextLog(ctx, "Iteration number: ", i)
		for _, rot := range []display.RotationAngle{
			display.Rotate90,
			display.Rotate180,
			display.Rotate270,
			display.Rotate0,
		} {
			testing.ContextLog(ctx, "Rotating to: ", rot)

			// Samples are grouped by vertical/horizontal rotation.
			keySuffix := "-horizontal"
			if rot == display.Rotate90 || rot == display.Rotate270 {
				keySuffix = "-vertical"
			}

			// Before resetting the stats, it is important to wait until the activity
			// is not generating new frame captures, otherwise it will generate "noise"
			// in the stats.
			if err := screen.WaitForStableFrames(ctx, a, pkgName); err != nil {
				return nil, err
			}

			if err := gfxinfoResetStats(ctx, a, pkgName); err != nil {
				return nil, err
			}

			if err := rotateDisplaySync(ctx, tconn, d, dispID, rot); err != nil {
				return nil, err
			}

			stats, err := screen.GfxinfoDumpStats(ctx, a, pkgName)
			if err != nil {
				return nil, err
			}
			numFrames := stats[screen.KeyTotalFramesRendered]
			if numFrames == 0 {
				testing.ContextLog(ctx, "Ignoring stats since no frames were captured during the screen rotation")
				continue
			}
			testing.ContextLogf(ctx, "Captured frames: %d", numFrames)

			for key, value := range stats {
				if err != nil {
					return nil, err
				}
				if key == screen.KeyJankyFrames {
					// Since "Janky frames" is meaningless by itself, we track the "Janky percentage" instead.
					key = "Janky percentage"
					value = 100 * value / numFrames
				}

				dir := perf.SmallerIsBetter
				if strings.HasPrefix(key, screen.KeyTotalFramesRendered) {
					dir = perf.BiggerIsBetter
				}

				unit := "count"
				if strings.Contains(key, "percentage") {
					unit = "percent"
				} else if strings.Contains(key, "percentile") {
					unit = "ms"
				}

				// Update 'key' at the latest to avoid interference with the comparisons.
				// perf/perf.go defines the list of valid chars for the metric name.
				key = key + keySuffix
				key = strings.Replace(key, " ", "_", -1)

				m := perf.Metric{
					Name:      key,
					Unit:      unit,
					Direction: dir,
					Multiple:  true,
				}
				samples[m] = append(samples[m], float64(value))
			}
		}
	}
	return samples, nil
}

// gfxinfoResetStats resets the graphics stats associated with a package name.
func gfxinfoResetStats(ctx context.Context, a *arc.ARC, pkgName string) error {
	return a.Command(ctx, "dumpsys", "gfxinfo", pkgName, "reset").Run()
}

// rotateDisplaySync rotates to display to a given angle. Waits until the rotation is complete in the Android side.
func rotateDisplaySync(ctx context.Context, tconn *chrome.TestConn, d *ui.Device, dispID string, rot display.RotationAngle) error {
	// Android rotations as defined in Surface.java
	// https://android.googlesource.com/platform/frameworks/base/+/refs/heads/android10-dev/core/java/android/view/Surface.java
	rots := map[int]display.RotationAngle{
		0: display.Rotate0,   // ROTATION_0
		1: display.Rotate90,  // ROTATION_90
		2: display.Rotate180, // ROTATION_180
		3: display.Rotate270, // ROTATION_270
	}

	// To be sure that rotation has finished we do:
	// - Start rotation from Ash.
	// - Wait until Android reports that it has the desired rotation.
	if err := display.SetDisplayRotationSync(ctx, tconn, dispID, rot); err != nil {
		return errors.Wrap(err, "failed to wait for display rotation")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := d.GetInfo(ctx)
		if err != nil {
			return err
		}
		if val, ok := rots[info.DisplayRotation]; !ok {
			return testing.PollBreak(errors.Errorf("unexpected rotation value: %v", info.DisplayRotation))
		} else if val != rot {
			return errors.Errorf("invalid rotation: want %q, got %q", rot, val)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to rotate device")
	}
	return nil
}
