// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosconfig provides methods for interacting with the cros_config
// command line utility. See https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/chromeos-config
// for more information about cros_config.
package crosconfig

import (
	"context"
	"strings"

	"chromiumos/tast/local/testexec"
)

// GetProperty returns the given property as a string if it is set and returns
// an empty string if it is not.
func GetProperty(ctx context.Context, path string, prop string) (string, error) {
	output, err := testexec.CommandContext(ctx, "cros_config", path, prop).Output(testexec.DumpLogOnError)
	status, ok := testexec.GetWaitStatus(err)
	if !ok {
		return "", err
	}

	// If cros_config exits with a code of 1, it means that the value was not
	// present in the model.yaml, and the output will be an empty string.
	if status.ExitStatus() == 0 || status.ExitStatus() == 1 {
		return strings.TrimSpace(string(output)), nil
	}

	return "", err
}
