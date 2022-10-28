// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Connector,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the camera service connector library works",
		Contacts: []string{
			"shik@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"arc_camera3", "chrome", caps.BuiltinOrVividCamera},
	})
}

func Connector(ctx context.Context, s *testing.State) {
	const exec = "cros_camera_connector_test"

	cr, err := testutil.WaitForCameraSocket(ctx)
	if err != nil {
		s.Fatal("Failed to wait for Camera Socket: ", err)
	}
	defer cr.Close(ctx)

	t := gtest.New(exec, gtest.Logfile(filepath.Join(s.OutDir(), "gtest.log")))

	if report, err := t.Run(ctx); err != nil {
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
		s.Errorf("Failed to run %v: %v", exec, err)
	}
}
