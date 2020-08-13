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

// BootedDeviceType is the set of mode,device,kernel verifier for the DUT.
type BootedDeviceType string

// The set of all possible BootedDeviceTypes, add more as needed.
const (
	BootedDeviceNormalInternalSig      BootedDeviceType = "normal mode booted from internal device, signature verified kernel key"
	BootedDeviceDeveloperInternalSig   BootedDeviceType = "developer mode booted from internal device, signature verified kernel key"
	BootedDeviceDeveloperRemovableSig  BootedDeviceType = "developer mode booted from removable device, signature verified kernel key"
	BootedDeviceDeveloperRemovableHash BootedDeviceType = "developer mode booted from removable device, hash verified kernel key"
)

// BootedDeviceInfo groups all the info required to determine the correct BootedDeviceType
type BootedDeviceInfo struct {
	MainfwType string
	Removable  bool
	KernvfyKey string
}

// knownBootedDeviceInfos contains all the valid BootedDeviceInfos, if a tuple is not here, then it is not recognized
var knownBootedDeviceInfos = map[BootedDeviceInfo]BootedDeviceType{
	BootedDeviceInfo{"normal", false, "sig"}:    BootedDeviceNormalInternalSig,
	BootedDeviceInfo{"developer", false, "sig"}: BootedDeviceDeveloperInternalSig,
	BootedDeviceInfo{"developer", true, "sig"}:  BootedDeviceDeveloperRemovableSig,
	BootedDeviceInfo{"developer", true, "hash"}: BootedDeviceDeveloperRemovableHash,
}

var (
	rTargetHosted    = regexp.MustCompile(`(?i)chrom(ium|e)os`)
	rDevNameStripper = regexp.MustCompile(`p?[0-9]+$`)
)

// BootedDevice reports the current BootedDeviceType of the DUT.
func (r *Reporter) BootedDevice(ctx context.Context) (BootedDeviceType, error) {
	cs, err := r.Crossystem(ctx, CrossystemParamMainfwType, CrossystemParamKernkeyVfy)
	if err != nil {
		return "", err
	}

	rootPart, err := getRootPartition(ctx, r)
	if err != nil {
		return "", errors.Wrap(err, "failed to get root partition")
	}

	removable, err := isRemovableDevice(ctx, r, rootPart)
	if err != nil {
		return "", errors.Wrapf(err, "failed to determine if %q is removable", rootPart)
	}

	cand := BootedDeviceInfo{cs[CrossystemParamMainfwType], removable, cs[CrossystemParamKernkeyVfy]}

	bootedDevice, ok := knownBootedDeviceInfos[cand]
	if !ok {
		return "", errors.Errorf("unrecognized BootedDeviceInfo: %v", cand)
	}

	return bootedDevice, nil
}

// getRootPartition gets the root partition as reported by the 'rootdev -s' command.
func getRootPartition(ctx context.Context, r *Reporter) (string, error) {
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

// isRemovableDevice determines if the device is removable media.
// TODO(aluo): deduplicate with utils.deviceRemovable.
func isRemovableDevice(ctx context.Context, r *Reporter, device string) (bool, error) {
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
