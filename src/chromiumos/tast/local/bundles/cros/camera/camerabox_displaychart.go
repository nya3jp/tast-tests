// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os/exec"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/camera/chart"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CameraboxDisplaychart,
		Desc:     "Verifies whether display chart script working normally",
		Contacts: []string{"beckerh@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func CameraboxDisplaychart(ctx context.Context, s *testing.State) {
	const Image = "/usr/share/chromeos-assets/wallpaper/child_small.jpg"
	testing.ContextLog(ctx, "display chart testing start")

	outdir := s.OutDir() + "/camerabox_displaychart.log"
	displayCmd := chart.DisplayCMD(Image, outdir)
	testing.ContextLog(ctx, "Start display chart process: ", displayCmd)

	// Run python command in background
	cmd := testexec.CommandContext(ctx, "sh", "-c", displayCmd)
	_, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLog(ctx, "display chart error")
	}
	//kill display chart PID
	defer unix.Kill(cmd.Process.Pid, unix.SIGKILL)

	//Polling log output and grep "Chart is ready" keyword to make sure chart display correctly
	testing.ContextLog(ctx, "Poll for 'chart is ready' message for ensuring chart is ready")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := testexec.CommandContext(ctx, "grep", chart.ChartReadyMsg, outdir).Output()
		switch err.(type) {
		case nil:
			return nil
		case *exec.ExitError:
			// Grep failed to find ready message, wait for next poll.
			return err
		default:
			return testing.PollBreak(err)
		}
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		testing.ContextLog(ctx, "Failed to wait for chart ready")
	}

	testing.ContextLog(ctx, "display chart testing done")
}
