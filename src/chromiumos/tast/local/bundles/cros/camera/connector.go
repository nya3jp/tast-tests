// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
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
	const socket = "/run/camera/camera3.sock"

	// TODO(b/151270948): Temporarily disable ARC when running this test.
	// The cros-camera service would kill itself when running the test if
	// arc_setup.cc is triggered at that time, which will fail the test.
	cr, err := chrome.New(ctx, chrome.ARCDisabled(), chrome.NoLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	if err := upstart.EnsureJobRunning(ctx, "cros-camera"); err != nil {
		s.Fatal("Failed to start cros-camera: ", err)
	}

	arcCameraGID, err := sysutil.GetGID("arc-camera")
	if err != nil {
		s.Fatal("Failed to get gid of arc-camera: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := os.Stat(socket)
		if err != nil {
			return err
		}
		perm := info.Mode().Perm()
		if perm != 0660 {
			return errors.Errorf("perm %04o (want %04o)", perm, 0660)
		}
		st := info.Sys().(*syscall.Stat_t)
		if st.Gid != arcCameraGID {
			return errors.Errorf("gid %04o (want %04o)", st.Gid, arcCameraGID)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		s.Fatal("Invalid camera socket: ", err)
	}

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
