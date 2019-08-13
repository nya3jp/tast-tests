// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenRotation,
		Desc:         "Verifies that Window Manager Critical User Journey behaves as described in go/arc-wm-p",
		Contacts:     []string{"arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ApiDemos.apk"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

func ScreenRotation(ctx context.Context, s *testing.State) {
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
	act, err := arc.NewActivity(a, pkgName, ".graphics.AnimateDrawables")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start ApiDemos activity: ", err)
	}

	// ApiDemos.apk is a PreN application. Requires that to manually put it in maximized state.
	if err := act.SetWindowState(ctx, arc.WindowStateMaximized); err != nil {
		s.Fatal("Failed to set change window state: ", err)
	}
	if err := act.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for idle activity: ", err)
	}

	// Wait for the "Application needs to restart to resize" dialog that appears on all Pre-N apks.
	if err := waitForRestartDialogAndRestart(ctx, act, d); err != nil {
		s.Fatal("Failed to restart Pre-N application: ", err)
	}

	if err := gfxinfoResetStats(ctx, a, act.PackageName()); err != nil {
		s.Fatal("Failed to reset gfxinfo: ", err)
	}

	// Leave Chromebook in reasonable state.
	rot0 := 0
	p := display.DisplayProperties{Rotation: &rot0}
	defer display.SetDisplayProperties(ctx, tconn, dispInfo.ID, p)

	re := regexp.MustCompile(
		`(Total frames rendered): (?P<num_frames>\d+)\s+` +
			`(Janky frames): (\d+) \((?:\d+\.\d+|-?nan)%\)\s+` +
			`(?:50th percentile: \d+ms\s+)?` + // Not present in M.
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
		wantedFramesPerSample       = 100
		samplesPer90DegreesRotation = 10
	)
	result := make(map[string][]int)
	groups := []string{}
	numFrames := 0

	for i := 0; i < samplesPer90DegreesRotation; i++ {
		s.Log("Iteration number: ", i)
		for _, rot := range []int{90, 180, 270, 0} {

			s.Logf("Rotating to: %d", rot)

			p := display.DisplayProperties{Rotation: &rot}
			if err := display.SetDisplayProperties(ctx, tconn, dispInfo.ID, p); err != nil {
				s.Fatal("Failed to set rotation: ", err)
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				stats, err := gfxinfoDumpStats(ctx, a, act.PackageName())
				if err != nil {
					s.Fatal("Failed to dump gfxinfo: ", err)
				}
				ss := string(stats)
				groups = re.FindStringSubmatch(ss)
				if len(groups) == 0 {
					s.Fatal("Failed to parse output: ", ss)
				}
				const numFramesGroupIdx = 2
				if numFrames, err = strconv.Atoi(groups[numFramesGroupIdx]); err != nil {
					return errors.Wrap(err, "failed to convert string to int")
				} else if numFrames < wantedFramesPerSample {
					return errors.Errorf("not enough frames: got %d; want %d", numFrames, wantedFramesPerSample)
				}

				// After a successful dump, reset stats
				return gfxinfoResetStats(ctx, a, act.PackageName())
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
				s.Fatal("Failed to grab a frame: ", err)
			}

			// Skip group 0, which is the one that contains all the groups together.
			for j := 1; j < len(groups); j++ {
				key := groups[j]
				if value, err := strconv.Atoi(groups[j+1]); err != nil {
					s.Fatal("Failed to convert string to int: ", err)
				} else {
					if key == "Janky frames" {
						// Since "Janky frames" is meaningless by itself, we track the "Janky percentage" instead.
						const jp = "Jankey percentage"
						result[jp] = append(result[jp], 100.0*value/numFrames)
					} else {
						result[key] = append(result[key], value)
					}
				}
				j++
			}
		}
	}

	// Convert parsed regexp into Perf data
	s.Logf("Result: %+v", result)

}

// gfxinfoDumpStats returns the gfxinfo stats associated with the package name.
func gfxinfoDumpStats(ctx context.Context, a *arc.ARC, pkgName string) ([]byte, error) {
	cmd := a.Command(ctx, "dumpsys", "gfxinfo", pkgName)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch dumpsys")
	}
	return output, nil
}

// gfxinfoResetStats resets the gfxinfo stats associated with the package name.
func gfxinfoResetStats(ctx context.Context, a *arc.ARC, pkgName string) error {
	return a.Command(ctx, "dumpsys", "gfxinfo", pkgName, "reset").Run()
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
	if err := obj.Click(ctx); err != nil {
		return errors.Wrap(err, "could not click on widget")
	}
	return act.WaitForIdle(ctx, 10*time.Second)
}
