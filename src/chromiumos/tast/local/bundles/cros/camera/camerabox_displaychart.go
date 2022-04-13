// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os/exec"
	"time"

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

	logFile := s.OutDir() + "/camerabox_displaychart.log"
	displayCmd := chart.DisplayCMD(Image, logFile)
	testing.ContextLog(ctx, "Start display chart process: ", displayCmd)

	//Run python command in background
	//The bash trick here is inevitable due to
	//the script will be run on another chart device instead of DUT without gRPC support.
	//So we cannot completely start python script in golang library
	//And since this test is kind of backing up for the remote test,
	//It also make sense to make them share more code as possible.

	cmd := testexec.CommandContext(ctx, "sh", "-c", displayCmd)
	if _, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		testing.ContextLog(ctx, "display chart error")
	}

	//kill display chart PID
	defer cmd.Kill()

	//Polling log output and grep "Chart is ready" keyword to make sure chart display correctly
	testing.ContextLog(ctx, "Poll for 'chart is ready' message for ensuring chart is ready")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := testexec.CommandContext(ctx, "grep", chart.ChartReadyMsg, logFile).Output()
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
