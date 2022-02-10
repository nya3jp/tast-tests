// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crossdevice

import (
	"context"
	"regexp"

	crossdevicecommon "chromiumos/tast/common/cros/crossdevice"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/lsbrelease"
)

// GetCrosAttributes gets the Chrome version and combines it into a CrosAttributes strct with the provided values for easy logging with json.MarshalIndent.
func GetCrosAttributes(ctx context.Context, tconn *chrome.TestConn, username string) (*crossdevicecommon.CrosAttributes, error) {
	attrs := crossdevicecommon.CrosAttributes{
		User: username,
	}
	out, err := testexec.CommandContext(ctx, "/opt/google/chrome/chrome", "--version").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get chrome version")
	}
	// The output on test images contains 'unknown' for the channel, i.e. 'Google Chrome 91.0.4435.0 unknown', so just extract the channel version.
	const versionPattern = `([0-9]+)\.([0-9]+)\.([0-9]+)\.([0-9]+)`
	r := regexp.MustCompile(versionPattern)
	versionMatch := r.FindStringSubmatch(string(out))
	if len(versionMatch) == 0 {
		return nil, errors.New("failed to find valid Chrome version")
	}
	attrs.ChromeVersion = versionMatch[1]

	lsb, err := lsbrelease.Load()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read lsb-release")
	}
	osVersion, ok := lsb[lsbrelease.Version]
	if !ok {
		return nil, errors.Wrap(err, "failed to read ChromeOS version from lsb-release")
	}
	attrs.ChromeOSVersion = osVersion

	board, ok := lsb[lsbrelease.Board]
	if !ok {
		return nil, errors.Wrap(err, "failed to read board from lsb-release")
	}
	attrs.Board = board

	model, err := testexec.CommandContext(ctx, "cros_config", "/", "name").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read model from cros_config")
	}
	attrs.Model = string(model)

	return &attrs, nil
}
