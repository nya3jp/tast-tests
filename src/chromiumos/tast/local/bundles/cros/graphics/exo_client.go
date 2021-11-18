// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExoClient,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Attaches a client to exo and exercises the wayland APIs",
		Contacts: []string{
			"jshargo@chromium.org",
			"chromeos-gfx-compositor@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_perbuild"},
		Fixture:      "chromeGraphics",
		SoftwareDeps: []string{"chrome", "no_qemu"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      5 * time.Minute,
	})
}

func ExoClient(ctx context.Context, s *testing.State) {
	testPath := filepath.Join(chrome.BinTestDir, "wayland_client_integration_tests")

	testList, err := gtest.ListTests(ctx, testPath)
	if err != nil {
		s.Fatal("Failed to list gtest: ", err)
	}

	for _, testcase := range testList {
		if report, err := gtest.New(
			testPath,
			gtest.Logfile(filepath.Join(s.OutDir(), testcase+".log")),
			gtest.Filter(testcase),
			gtest.ExtraArgs("--test-launcher-jobs=1", "--use-drm", "--wayland_socket=/var/run/chrome/wayland-0"),
			gtest.UID(int(sysutil.ChronosUID)),
		).Run(ctx); err != nil {
			s.Errorf("%s failed: %v", testcase, err)
			if report != nil {
				for _, name := range report.FailedTestNames() {
					s.Error(name, " failed")
				}
			}
		}
	}
}
