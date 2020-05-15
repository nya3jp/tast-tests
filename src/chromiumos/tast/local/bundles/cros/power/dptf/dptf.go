// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dptf implements helpers for the power.dptf* tests.
package dptf

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// GetProfileFromOverrideScript checks existence of dptf_override.sh and calls
// the script to get the expected DPTF profile.
func GetProfileFromOverrideScript(ctx context.Context) (string, error) {
	if _, err := os.Stat("/etc/dptf/dptf_override.sh"); err != nil {
		if os.IsNotExist(err) {
			return "", errors.Wrap(err, "can't find DPTF override script")
		}
		return "", errors.Wrap(err, "unexpected os.Stat error")
	}

	out, err := testexec.CommandContext(ctx, "sh", "-c", ". /etc/dptf/dptf_override.sh; dptf_get_override").Output()
	if err != nil {
		return "", errors.Wrap(err, "error in DPTF override script")
	}
	if len(out) == 0 {
		return "", errors.Wrap(err, "can't find DPTF profile from override script")
	}
	return filepath.Join("/etc/dptf/", strings.TrimSpace(string(out))), nil
}

// GetProfileFromCrosConfig calls cros_config to get the expected DPTF profile.
func GetProfileFromCrosConfig(ctx context.Context) (string, error) {
	out, err := testexec.CommandContext(ctx, "cros_config", "/thermal", "dptf-dv").Output()
	if err != nil {
		// cros_config return non-zero when expecting no DPTF profile.
		return "", nil
	}
	if len(out) == 0 {
		return "", errors.Wrap(err, "can't find DPTF profile via cros_config")
	}
	return filepath.Join("/etc/dptf/", strings.TrimSpace(string(out))), nil
}

// GetProfileFromPgrep uses pgrep to get DPTF profile currently in use.
func GetProfileFromPgrep(ctx context.Context) (string, error) {
	out, err := testexec.CommandContext(ctx, "pgrep", "-a", "esif_ufd").Output()
	if err != nil {
		return "", errors.Wrap(err, "search for DPTF process failed")
	}
	if len(out) == 0 {
		return "", errors.Wrap(err, "can't find DPTF process")
	}
	outSlice := strings.Fields(string(out))
	last := outSlice[len(outSlice)-1]
	if strings.HasPrefix(last, "/etc/dptf/") {
		return last, nil
	}
	return "", nil
}
