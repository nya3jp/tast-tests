// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/remote/sysutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckMakeRootfsWritable,
		Desc: "Test that we can make rootfs writable",
		Contacts: []string{
			"tomhughes@chromium.org",
			"josienordrum@google.com",
			"chromeos-fingerprint@google.com",
		},
		// TODO(b/256910247): re-add test to group:mainline once /b/249564120 is fixed.
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService", dutfs.ServiceName},
		Vars:         []string{"servo"},
	})
}

func CheckMakeRootfsWritable(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer d.Close(ctx)

	servoSpec, ok := s.Var("servo")
	if !ok {
		servoSpec = ""
	}
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	rootfsIsWritable, err := sysutil.IsRootfsWritable(ctx, d.RPC())
	if err != nil {
		s.Fatal("Failed to check if rootfs is writable: ", err)
	}
	if rootfsIsWritable {
		s.Fatal("WARNING: The rootfs was already writable")
	} else {
		// Since MakeRootfsWritable will reboot the device, we must call
		// RPCClose/RPCDial before/after calling MakeRootfsWritable.
		if err := d.RPCClose(ctx); err != nil {
			s.Fatal("Failed to close rpc: ", err)
		}
		// Note that there is no way for this test to restore the rootfs back to read-only mode.
		// b/249564120 ensures that the test infrastructure will do so.
		if err := sysutil.MakeRootfsWritable(ctx, d.DUT(), d.RPCHint()); err != nil {
			s.Fatal("Failed to make rootfs writable: ", err)
		}
		if err := d.RPCDial(ctx); err != nil {
			s.Fatal("Failed to redial rpc: ", err)
		}
	}

	fs := dutfs.NewClient(d.RPC().Conn)
	filename := "/testfile.txt"
	content := []byte("This is a test file!")
	if err := fs.WriteFile(ctx, filename, content, 0); err != nil {
		s.Fatal("Failed to create test file: ", err)
	}
	if err := fs.RemoveAll(ctx, filename); err != nil {
		s.Fatal("Failed to remove test file: ", err)
	}
}
