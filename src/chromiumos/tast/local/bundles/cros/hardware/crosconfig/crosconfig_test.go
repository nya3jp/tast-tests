// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crosconfig

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
)

const crosConfigChild = "CROSCONFIG_CHILD"

func init() {
	if os.Getenv(crosConfigChild) == "1" {
		// child process
		crosConfig(os.Args[1:])
		panic("unreachable")
	}
}

func TestCheckHardwareProperty(t *testing.T) {
	origCrosConfigGet := runGet
	runGet = fakeCrosConfigGet
	defer func() { runGet = origCrosConfigGet }()

	for _, tc := range []struct {
		prop        HardwareProperty
		expected    bool
		expectError bool
	}{
		{HasBaseGyroscope, false, false},
		{HasBaseMagnetometer, false, false},
		{HasBaseAccelerometer, false, true},
		{HasLidAccelerometer, true, false},
		{HasLidGyroscope, false, true},
	} {
		out, err := CheckHardwareProperty(context.Background(), tc.prop)

		if err != nil {
			if crosconfig.IsNotFound(err) {
				t.Errorf("[%v] Unexpected error, got %v", tc.prop, err)
			}
			if !tc.expectError {
				t.Errorf("[%v] Expected no error, got %v", tc.prop, err)
			}
		} else if out != tc.expected {
			t.Errorf("[%v] Expected out to be %v, got %v", tc.prop, tc.expected, out)
		}
	}
}

func fakeCrosConfigGet(ctx context.Context, path, prop string) (string, error) {
	b, err := fakeRunCrosConfig(ctx, path, prop)
	if err != nil {
		status, ok := testexec.GetWaitStatus(err)
		if ok && status.ExitStatus() == 1 {
			return "", &crosconfig.ErrNotFound{E: errors.Errorf("property not found: %v", prop)}
		}

		return "", err
	}

	return strings.TrimSpace(string(b)), nil
}

func fakeRunCrosConfig(ctx context.Context, args ...string) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, errors.Wrap(err, "error getting current process")
	}

	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = append(os.Environ(), crosConfigChild+"=1")
	return cmd.Output()
}

// crosConfig is a fake cros_config main function.
func crosConfig(args []string) {
	if len(args) < 2 {
		panic("not enough args")
	}

	prop := args[1]

	switch prop {
	case string(HasBaseGyroscope):
		os.Exit(1)
	case string(HasBaseMagnetometer):
		fmt.Fprintf(os.Stdout, "false")
	case string(HasBaseAccelerometer):
		fmt.Fprintf(os.Stdout, "")
	case string(HasLidGyroscope):
		os.Exit(2)
	default:
		fmt.Fprintf(os.Stdout, "true")
	}

	os.Exit(0)
}
