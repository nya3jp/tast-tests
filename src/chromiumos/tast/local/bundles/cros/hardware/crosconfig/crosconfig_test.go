// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crosconfig

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

const crosConfigChild = "CROSCONFIG_CHILD"

func TestCheckHardwareProperty(t *testing.T) {
	origRunCrosConfig := runCrosConfig
	runCrosConfig = fakeRunCrosConfig
	defer func() { runCrosConfig = origRunCrosConfig }()

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

		if err != nil && !tc.expectError {
			t.Errorf("[%v] Expected no error, got %v", tc.prop, err)
		} else if out != tc.expected {
			t.Errorf("[%v] Expected out to be %v, got %v", tc.prop, tc.expected, out)
		}
	}
}

func fakeRunCrosConfig(ctx context.Context, args ...string) ([]byte, error) {
	cs := []string{"--"}
	cs = append(cs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = []string{crosConfigChild + "=1"}
	return cmd.Output()
}

func init() {
	if os.Getenv(crosConfigChild) == "1" {
		// child process
		crosConfig(os.Args[2:]...)
		panic("unreachable")
	}
}

// Fake cros_config
func crosConfig(args ...string) {
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
