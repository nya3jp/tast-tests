// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crossdevice

import (
	"context"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/systemlogs"
	"chromiumos/tast/lsbrelease"
)

// CrosAttributes contains information about the CrOS device that are relevant to Nearby Share.
type CrosAttributes struct {
	User            string
	ChromeVersion   string
	ChromeOSVersion string
	Board           string
	Model           string
}

// GetCrosAttributes gets the Chrome version and combines it into a CrosAttributes strct with the provided values for easy logging with json.MarshalIndent.
func GetCrosAttributes(ctx context.Context, tconn *chrome.TestConn, username string) (*CrosAttributes, error) {
	attrs := CrosAttributes{
		User: username,
	}
	const expectedKey = "CHROME VERSION"
	version, err := systemlogs.GetSystemLogs(ctx, tconn, expectedKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting system logs to check Chrome version")
	}
	if version == "" {
		return nil, errors.Wrap(err, "system logs result empty")
	}
	// The output on test images contains 'unknown' for the channel, i.e. '91.0.4435.0 unknown', so just extract the channel version.
	const versionPattern = `([0-9\.]+) [\w+]`
	r, err := regexp.Compile(versionPattern)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile Chrome version pattern")
	}
	versionMatch := r.FindStringSubmatch(version)
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
