// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CameraboxDisplaychart,
		Desc:     "Verifies whether display chart script working normally ",
		Contacts: []string{"beckerh@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func CameraboxDisplaychart(ctx context.Context, s *testing.State) {
	var pid string
	const chartReadyMsg = "Chart is ready."
	const Script = "/usr/local/autotest/bin/display_chart.py"
	const Image = "/usr/share/chromeos-assets/wallpaper/child_small.jpg"
	const OutputLog = "/tmp/camerabox_displaychart.log"

	testing.ContextLog(ctx, "display chart testing start")

	//python command
	displayCmd := fmt.Sprintf(
		"(python %s %s > %s 2>&1) & echo -n $!",
		shutil.Escape(Script), shutil.Escape(Image),
		shutil.Escape(OutputLog))

	testing.ContextLog(ctx, "Start display chart process: ", displayCmd)

	// Run python command in background
	cmd := testexec.CommandContext(ctx, "sh", "-c", displayCmd)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLog(ctx, "display chart error")
	}
	//get pid
	pid = strings.TrimSpace(string(out))

	//polling , grep keyword
	testing.ContextLog(ctx, "Poll for 'chart is ready' message for ensuring chart is ready")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := testexec.CommandContext(ctx, "grep", chartReadyMsg, OutputLog).Output()
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

	//write command to dedicate file
	writefile := fmt.Sprintf("cp %s %s", OutputLog, s.OutDir())
	testing.ContextLog(ctx, "write file :", writefile)
	_, err = testexec.CommandContext(ctx, "sh", "-c", writefile).Output()
	if err != nil {
		testing.ContextLog(ctx, "write file error:", err.Error())
	}

	//kill display chart PID
	if err := testexec.CommandContext(ctx, "kill", "-2", pid).Run(); err != nil {
		testing.ContextLog(ctx, "Failed to send interrupt signal to close display script")
	}

	testing.ContextLog(ctx, "display chart testing done")
}
