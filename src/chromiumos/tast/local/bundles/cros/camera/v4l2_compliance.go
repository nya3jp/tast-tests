// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"os"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: V4L2Compliance,
		Desc: "Runs V4L2Compliance in all the Media Devices",
		Contacts: []string{
			"ribalda@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func V4L2Compliance(ctx context.Context, s *testing.State) {
	fileExists := func(file string) bool {
		_, err := os.Stat(file)
		return !os.IsNotExist(err)
	}
	testOutput := ""

	for i := 0; ; i++ {
		testing.ContextLog(ctx, "Testing: ", i)

		videodev := fmt.Sprintf("/dev/video%v", i)
		if !fileExists(videodev) {
			break
		}

		cmd := testexec.CommandContext(ctx, "v4l2-compliance", "-v", "-d", videodev)
		out, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil && cmd.ProcessState.ExitCode() != 1 {
			s.Fatal("Error running v4l2-compliance")
		}

		result := string(out)
		testing.ContextLog(ctx, result)

		if err == nil {
			continue
		}

		//Remove last end of line if present
		result = strings.TrimSuffix(result, "\n")
		//Get last line
		result = result[strings.LastIndex(result, "\n"):]
		testOutput += result
	}

	if testOutput != "" {
		s.Error(testOutput)
	}

}
