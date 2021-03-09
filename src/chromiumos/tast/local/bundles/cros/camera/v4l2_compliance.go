// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"
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

	mediaDevices, err := filepath.Glob("/dev/v4l/by-path/*")
	if err != nil {
		s.Fatal("Failed to list Media Devices: ", err)
	}

	for _, videodev := range mediaDevices {
		cmd := testexec.CommandContext(ctx, "v4l2-compliance", "-v", "-d", videodev)
		out, err := cmd.Output(testexec.DumpLogOnError)

		if err == nil {
			continue
		}

		// Log full output on error.
		result := string(out)
		s.Log(result)

		if cmd.ProcessState.ExitCode() != 1 {
			s.Fatalf("v4l2-compliance failed: %s: %v", videodev, err)
		}

		// Remove last end of line if present.
		result = strings.TrimSuffix(result, "\n")
		// Get last line.
		lastline := result[strings.LastIndex(result, "\n"):]
		s.Errorf("v4l2-compliance failed: %s: %s", videodev, lastline)
	}
}
