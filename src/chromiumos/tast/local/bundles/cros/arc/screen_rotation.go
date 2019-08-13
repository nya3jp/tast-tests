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
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

func ScreenRotation(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get a Test API connection: ", err)
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for idle CPU: ", err)
	}

	if _, err := gfxinfoDumpAndReset(ctx, a, act); err != nil {
		s.Fatal("Failed to reset gfxinfo: ", err)
	}

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

	const samplesPer90DegreesRotation = 10
	result := make(map[string][]int)

	for i := 0; i < samplesPer90DegreesRotation; i++ {
		s.Log("Iteration number: ", i)
		for _, rot := range []int{0, 90, 180, 270} {

			s.Logf("Rotating to: %d", rot)

			p := display.DisplayProperties{Rotation: &rot}
			if err := display.SetDisplayProperties(ctx, tconn, dispInfo.ID, p); err != nil {
				s.Fatal("Failed to set rotation: ", err)
			}

			// The original cheets_ScreenRotation was using an "sleep(5)" here.
			// Based on same tests, it seems to be safe to replace it with "act.WaitForIdle()".
			if err := act.WaitForIdle(ctx, 10*time.Second); err != nil {
				s.Fatal("Failed to wait for idle activity: ", err)
			}

			info, err := gfxinfoDumpAndReset(ctx, a, act)
			if err != nil {
				s.Fatal("Failed to dump gfxinfo: ", err)
			}
			groups := re.FindStringSubmatch(string(info))
			if len(groups) == 0 {
				s.Fatal("Failed to parse output: ", string(info))
			}

			const numFramesGroupIdx = 2
			if numFrames, err := strconv.Atoi(groups[numFramesGroupIdx]); err != nil {
				s.Fatal("Failed to convert string to int: ", err)
			} else {
				if numFrames == 0 {
					s.Logf("Invalid number of frames in iteration %d, rotation %d. Skipping sample", i, rot)
					continue
				}
			}

			// Skip group 0, which is the one that contains all the groups together.
			for j := 1; j < len(groups); j++ {
				key := groups[j]
				if value, err := strconv.Atoi(groups[j+1]); err != nil {
					s.Fatal("Failed to convert string to int: ", err)
				} else {
					result[key] = append(result[key], value)
				}
				j++
			}
		}
	}
	s.Logf("Result: %+v", result)
}

// gfxinfoDumpAndReset returns the gfxinfo associated with the activity and resets its stats.
func gfxinfoDumpAndReset(ctx context.Context, a *arc.ARC, act *arc.Activity) ([]byte, error) {
	testing.Sleep(ctx, 200*time.Millisecond)
	cmd := a.Command(ctx, "dumpsys", "gfxinfo", act.PackageName(), "reset")
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch dumpsys")
	}
	testing.Sleep(ctx, 200*time.Millisecond)
	return output, nil
}
