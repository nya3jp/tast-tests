// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosconfig provides methods for using the cros_config command line
// utility. See https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/chromeos-config
// for more information about cros_config.
package crosconfig

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// ErrNotFound describes an error encountered when a property is not found by
// cros_config.
type ErrNotFound struct {
	*errors.E
}

// IsNotFound returns true if the given error is of type errNotFound.
func IsNotFound(err error) bool {
	_, ok := err.(*ErrNotFound)
	return ok
}

// Get returns the given property as a string if it is set and returns an empty
// string if it is not.
func Get(ctx context.Context, path, prop string) (string, error) {
	b, err := testexec.CommandContext(ctx, "cros_config", path, prop).Output()
	if err != nil {
		status, ok := testexec.GetWaitStatus(err)
		if ok && status.ExitStatus() == 1 {
			return "", &ErrNotFound{E: errors.Errorf("property not found: %v", prop)}
		}

		return "", err
	}

	return strings.TrimSpace(string(b)), nil
}
