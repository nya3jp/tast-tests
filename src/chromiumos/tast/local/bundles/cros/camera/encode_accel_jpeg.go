// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"path/filepath"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelJPEG,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Run Chrome jpeg_encode_accelerator_unittest",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeJPEG},
		Data:         []string{"bali_640x368_P420.yuv"},
	})
}

// EncodeAccelJPEG runs a set of HW JPEG encode tests, defined in
// jpeg_encode_accelerator_unittest.
func EncodeAccelJPEG(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// Stopping the UI is not strictly needed to run the test executable
	// below. However, it's a good idea for stability reasons: if the UI
	// plays a video (e.g., in the OOBE), we don't want that to interfere
	// with the test.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Execute the test binary.
	const exec = "jpeg_encode_accelerator_unittest"
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), "gtest.log")),
		gtest.ExtraArgs(
			logging.ChromeVmoduleFlag(),
			fmt.Sprintf("--yuv_filenames=%s:640x368", s.DataPath("bali_640x368_P420.yuv"))),
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
