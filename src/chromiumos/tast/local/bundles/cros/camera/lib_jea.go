// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"path/filepath"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LibJEA,
		Desc: "Runs cros-camera-libjea_test to make sure jea works on Chrome OS side",
		Contacts: []string{
			"wtlee@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"arc_camera3", "chrome", caps.HWEncodeJPEG},
		Data:         []string{"bali_640x360_P420.yuv", "lake_4160x3120_P420.yuv"},
	})
}

func LibJEA(ctx context.Context, s *testing.State) {
	const exec = "libjea_test"
	inputArg1 := fmt.Sprintf("--yuv_filename1=%s:640x360", s.DataPath("bali_640x360_P420.yuv"))
	inputArg2 := fmt.Sprintf("--yuv_filename2=%s:4160x3120", s.DataPath("lake_4160x3120_P420.yuv"))

	if report, err := gtest.New(
		exec,
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.ExtraArgs(inputArg1, inputArg2, logging.ChromeVmoduleFlag()),
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
