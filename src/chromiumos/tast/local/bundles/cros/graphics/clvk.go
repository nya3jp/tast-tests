// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

// clvkTest is used to describe the config used to run each test.
type clvkTest struct {
	exe string // The test executable name.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Clvk,
		Desc: "Run OpenCL implementation on top of Vulkan using clvk",
		Contacts: []string{
			"rjodin@chromium.org",
			"chromeos-gfx@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_opencl", "graphics_nightly"},
		SoftwareDeps: []string{"vulkan"},
		Fixture:      "graphicsNoChrome",
		Params: []testing.Param{{
			Name: "api_tests",
			Val: clvkTest{
				exe: "api_tests",
			},
			Timeout: 1 * time.Minute,
		}, {
			Name: "simple_test",
			Val: clvkTest{
				exe: "simple_test",
			},
			Timeout: 1 * time.Minute,
		}},
	})
}

func Clvk(ctx context.Context, s *testing.State) {
	test := s.Param().(clvkTest)

	const testPath = "/usr/local/opencl"
	testExec := filepath.Join(testPath, test.exe)
	logFile := filepath.Join(s.OutDir(), filepath.Base(test.exe)+".txt")
	if test.exe == "api_tests" {
		if report, err := gtest.New(testExec,
			gtest.Logfile(logFile),
		).Run(ctx); err != nil && report != nil {
			passedTests := report.PassedTestNames()
			failedTests := report.FailedTestNames()
			s.Errorf("Passed %d tests, failed %d tests (%s)",
				len(passedTests), len(failedTests), failedTests)
		} else if err != nil && report == nil {
			s.Fatal("Failed to run api_tests: ", err)
		}
	} else {
		f, err := os.Create(logFile)
		if err != nil {
			s.Fatal("Failed to create a log file: ", err)
		}
		defer f.Close()

		cmd := testexec.CommandContext(ctx, testExec)
		cmd.Stdout = f
		cmd.Stderr = f
		if err = cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to run ", testExec, ": ", err)
		}
	}
}
