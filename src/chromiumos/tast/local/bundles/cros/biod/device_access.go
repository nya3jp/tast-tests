// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package biod

import (
	"context"
	"os"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceAccess,
		Desc: "Checks that /dev/cros_fp can be accessed by the biod user",
		Contacts: []string{
			"hesling@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
	})
}

// DeviceAccess checks that /dev/cros_fp can be accessed by the biod user.
//
// This ensures that /dev/cros_fp exists, udev setup minimal permissions,
// and that system groups are configured properly.
//
// Note that this test does not check that the /dev/cros_fp device is properly
// locked down to a minimal group.
func DeviceAccess(ctx context.Context, s *testing.State) {
	if _, err := os.Stat("/dev/cros_fp"); err != nil {
		if os.IsNotExist(err) {
			s.Fatal("The /dev/cros_fp file doesn't exist")
		}
		s.Fatal("Failed to stat /dev/cros_fp: ", err)
	}

	var tests = []struct {
		name   string
		testOp string
		errMsg string
	}{
		{"IsCharDev", "-c", "The /dev/cros_fp file is not a char device"},
		{"IsReadable", "-r", "The biod user doesn't have read access to /dev/cros_fp"},
		{"IsWritable", "-w", "The biod user doesn't have write access to /dev/cros_fp"},
	}

	for _, tc := range tests {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			result, err := testAsCommand(ctx, "biod", "test", tc.testOp, "/dev/cros_fp")
			if err != nil {
				s.Fatalf("Failed to execute 'test %s /dev/cros_fp': %v", tc.testOp, err)
			}
			if !result {
				s.Fatal(tc.errMsg)
			}
		})
	}
}

// testAsCommand runs the test cmd as the given user and reports
// true or false, based on its exit code.
//
// The biod user indirectly has access to /dev/cros_fp through its
// group, thus we use "test" instead of directly checking the file
// permission.
func testAsCommand(ctx context.Context, user string, testCmd ...string) (bool, error) {
	cmd, err := testexec.CommandContextUser(
		ctx,
		user,
		testCmd[0],
		testCmd[1:]...,
	)
	if err != nil {
		return false, err
	}

	err = cmd.Run()
	if err != nil {
		_, ok := testexec.ExitCode(err)
		if !ok {
			return false, errors.Wrap(err, "failed to run command")
		}
		return false, nil
	}
	return true, nil
}
