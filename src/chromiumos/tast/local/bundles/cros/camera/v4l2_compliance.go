// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
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

func listAllMediaDevices() ([]string, error) {
	var matches []string

	err := filepath.Walk("/dev/v4l/by-path/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		matches = append(matches, path)
		return nil
	})
	return matches, err
}

func V4L2Compliance(ctx context.Context, s *testing.State) {
	testOutput := ""

	mediaDevices, err := listAllMediaDevices()
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
			s.Fatalf("Error running v4l2-compliance, ret=%v", cmd.ProcessState.ExitCode())
		}

		// Remove last end of line if present.
		result = strings.TrimSuffix(result, "\n")
		// Get last line.
		result = result[strings.LastIndex(result, "\n"):]
		testOutput += result
	}

	if testOutput != "" {
		s.Error(testOutput)
	}

}
