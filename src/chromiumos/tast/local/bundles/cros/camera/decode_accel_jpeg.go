// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelJPEG,
		Desc:         "Run Chrome jpeg_decode_accelerator_unittest",
		Contacts:     []string{"henryhsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeJPEG},
		Data:         []string{decodeAccelJpegTestFile},
	})
}

const decodeAccelJpegTestFile = "peach_pi-1280x720.jpg"

// DecodeAccelJPEG runs a set of HW JPEG decode tests, defined in
// jpeg_decode_accelerator_unittest.
func DecodeAccelJPEG(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	testDir := filepath.Dir(s.DataPath(decodeAccelJpegTestFile))

	// Stopping the UI is not strictly needed to run the test executable
	// below. However, it's a good idea for stability reasons: if the UI
	// plays a video (e.g., in the OOBE), we don't want that to interfere
	// with the test.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Execute the test binary.
	const exec = "jpeg_decode_accelerator_unittest"
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.ExtraArgs(
			logging.ChromeVmoduleFlag(),
			"--test_data_path="+testDir+"/",
			"--jpeg_filenames="+decodeAccelJpegTestFile),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
	}
}
