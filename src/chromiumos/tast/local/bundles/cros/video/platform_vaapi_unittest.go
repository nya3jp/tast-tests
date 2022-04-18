// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PlatformVAAPIUnittest,
		Desc: "Runs test_va_api, a shallow libva API test",
		Contacts: []string{
			"stevecho@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"vaapi"},
		// TODO(b/191801955): Reenable on grunt when it stops hanging forever.
		HardwareDeps: hwdep.D(hwdep.SkipOnPlatform("grunt")),
	})
}

// PlatformVAAPIUnittest runs the "test_va_api" GTtest binary
// from the libva-test package,
// see https://github.com/intel/libva-utils.
func PlatformVAAPIUnittest(ctx context.Context, s *testing.State) {
	const exec = "test_va_api"
	if report, err := gtest.New(exec,
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		s.Errorf("%v failed: %v", exec, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		} else {
			s.Error("No additional information is available for this failure")
		}
	}
}
