// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package biod

import (
	"context"
	"strings"

	"chromiumos/tast/common/errors"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DevicePermission,
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

// DevicePermission checks that /dev/cros_fp can be accessed by the biod user.
//
// The reason why biod might not be able to access /dev/cros_fp is
// if the file permissions we not setup correctly in udev or
// if groups werre not configured properly.
func DevicePermission(ctx context.Context, s *testing.State) {
	// Check if the char device exists, otherwise all remaining tests
	// will fail incorrectly.
	result, err := biodTestCommand(ctx, "test -c /dev/cros_fp")
	if err != nil {
		s.Fatal("Failed to test if /dev/cros_fp char device exists", err)
	}
	if !result {
		s.Fatal("The /dev/cros_fp char device doesn't exist")
	}

	// Check if the char device is readable.
	result, err = biodTestCommand(ctx, "test -r /dev/cros_fp")
	if err != nil {
		s.Fatal("Failed to test /dev/cros_fp for read access: ", err)
	}
	if !result {
		s.Error("The biod user doesn't have read access to /dev/cros_fp")
	}

	// Check if the char device is writable.
	result, err = biodTestCommand(ctx, "test -w /dev/cros_fp")
	if err != nil {
		s.Fatal("Failed to test /dev/cros_fp for write access: ", err)
	}
	if !result {
		s.Error("The biod user doesn't have write access to /dev/cros_fp")
	}
}

// biodTestCommand runs the test cmd as the biod user and reports
// true or false, based on its exit code.
//
// The biod user indirectly has access to /dev/cros_fp through its
// group, thus we use "test" instead of directly checking the file
// permission.
func biodTestCommand(ctx context.Context, testCmd string) (bool, error) {
	cmd := []string{
		"su",
		// The default shell for biod is /bin/false, which would
		// make any command passed to it fail.
		"--shell", "/bin/bash",
		// Echo yes and no to distinguish a su error vs test result.
		"--command", testCmd + " && echo yes || echo no",
		// Run as biod user.
		"biod",
	}
	out, err := testexec.CommandContext(
		ctx,
		cmd[0],
		cmd[1:]...,
	).CombinedOutput()

	if err != nil {
		testing.ContextLogf(ctx, "Failed to run command %q", strings.Join(cmd, " "))
		testing.ContextLogf(ctx, "Received %q", string(out))
		return false, err
	}

	switch {
	case strings.Contains(string(out), "no"):
		return false, nil
	case strings.Contains(string(out), "yes"):
		return true, nil
	default:
		return false, errors.New("output didn't contain a yes or no")
	}
}
