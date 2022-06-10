// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
)

var (
	rTargetHosted    = regexp.MustCompile(`(?i)chrom(ium|e)os`)
	rDevNameStripper = regexp.MustCompile(`p?[0-9]+$`)
)

// BootedFromRemovableDevice returns true if the root partition is on a removable device.
func (r *Reporter) BootedFromRemovableDevice(ctx context.Context) (bool, error) {
	rootPart, err := RootPartition(ctx, r)
	if err != nil {
		return false, errors.Wrap(err, "failed to get root partition")
	}

	removable, err := IsRemovableDevice(ctx, r, rootPart)
	if err != nil {
		return false, errors.Wrapf(err, "failed to determine if %q is removable", rootPart)
	}
	return removable, nil
}

// RootPartition gets the root partition as reported by the 'rootdev -s' command.
func RootPartition(ctx context.Context, r *Reporter) (string, error) {
	lines, err := r.CommandOutputLines(ctx, "rootdev", "-s")
	if err != nil {
		return "", errors.Wrap(err, "failed to determine root partition")
	}

	if len(lines) == 0 {
		return "", errors.New("root partition not found")
	}

	return lines[0], nil
}

// isTargetHosted determines if DUT is hosted by checking if /etc/lsb-release has chromiumos attributes.
func isTargetHosted(ctx context.Context, r *Reporter) (bool, error) {
	const targetHostedFile = "/etc/lsb-release"
	lines, err := r.CatFileLines(ctx, targetHostedFile)
	if err != nil {
		return false, err
	}

	// If file is empty, then it's some kind of system error.
	if len(lines) == 0 {
		return false, nil
	}
	return rTargetHosted.FindStringIndex(lines[0]) != nil, nil
}

// IsRemovableDevice determines if the device is removable media.
// TODO(aluo): deduplicate with utils.deviceRemovable.
func IsRemovableDevice(ctx context.Context, r *Reporter, device string) (bool, error) {
	hosted, err := isTargetHosted(ctx, r)
	if err != nil {
		return false, err
	}

	if !hosted {
		return false, nil
	}

	// Removes the partition portion of the device.
	baseDev := rDevNameStripper.ReplaceAllString(strings.Split(device, "/")[2], "")

	removable, err := r.CatFile(ctx, fmt.Sprintf("/sys/block/%s/removable", baseDev))
	if err != nil {
		return false, err
	}

	if removable != "0" && removable != "1" {
		return false, errors.Wrapf(err, "removable output %q is not 0 or 1", removable)
	}
	return removable == "1", nil
}
