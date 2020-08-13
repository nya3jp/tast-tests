// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// BootedDeviceType is the set of mode,device,kernel verifier for the DUT.
type BootedDeviceType string

// The set of all possible BootedDeviceTypes, add more as needed.
const (
	BootedDeviceTypeNormalInternalSig      BootedDeviceType = "normal mode booted from internal device, signature verified kernel key"
	BootedDeviceTypeDeveloperInternalSig   BootedDeviceType = "developer mode booted from internal device, signature verified kernel key"
	BootedDeviceTypeDeveloperRemovableSig  BootedDeviceType = "developer mode booted from removable device, signature verified kernel key"
	BootedDeviceTypeDeveloperRemovableHash BootedDeviceType = "developer mode booted from removable device, hash verified kernel key"
)

// lookupTuple groups all the info required to determine the correct BootedDeviceType
type lookupTuple struct {
	mode      string
	removable bool
	vfy       string
}

// verificationMap contains all the valid lookupTuples, if a tuple is not here, then it is not recognized
var verificationMap = map[lookupTuple]BootedDeviceType{
	lookupTuple{"normal", false, "sig"}:    BootedDeviceTypeNormalInternalSig,
	lookupTuple{"developer", false, "sig"}: BootedDeviceTypeDeveloperInternalSig,
	lookupTuple{"developer", true, "sig"}:  BootedDeviceTypeDeveloperRemovableSig,
	lookupTuple{"developer", true, "hash"}: BootedDeviceTypeDeveloperRemovableHash,
}

var rTargetHosted = regexp.MustCompile(`(?i)chrom(ium|e)os`)
var rDevNameStripper = regexp.MustCompile(`p?[0-9]+$`)

// BootedDevice reports the current BootedDeviceType of the DUT.
func (r *Reporter) BootedDevice(ctx context.Context) (BootedDeviceType, error) {
	cs, err := r.Crossystem(ctx, CrossystemKeyMainfwType, CrossystemKeyKernkeyVfy)
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

	bootedDevice, found := verificationMap[lookupTuple{cs[CrossystemKeyMainfwType], removable, cs[CrossystemKeyKernkeyVfy]}]
	if !found {
		return "", errors.Errorf("unrecognized combination of %v=%v, %v=%v, %v=%v", CrossystemKeyMainfwType, cs[CrossystemKeyMainfwType], "removable", removable, CrossystemKeyKernkeyVfy, cs[CrossystemKeyKernkeyVfy])
	}

	return bootedDevice, nil
}

// getRootPartition gets the root partition as reported by the 'rootdev -s' command.
func getRootPartition(ctx context.Context, r *Reporter) (string, error) {
	res, err := r.CommandOutputLines(ctx, "rootdev", "-s")
	if err != nil {
		return "", errors.Wrap(err, "failed to determine root partition")
	}

	if len(res) == 0 {
		return "", errors.New("root partition not found")
	}

	return res[0], nil
}

// isTargetHosted determines if DUT is hosted by checking if /etc/lsb-release has chromiumos attributes.
func isTargetHosted(ctx context.Context, r *Reporter) (bool, error) {
	const targetHostedFile = "/etc/lsb-release"
	res, err := r.CatFileLines(ctx, targetHostedFile)
	if err != nil {
		return false, err
	}

	if len(res) == 0 {
		return false, nil
	}
	return rTargetHosted.FindStringIndex(res[0]) != nil, nil
}

// isRemovableDevice determines if DUT is hosted by checking if /etc/lsb-release has chromiumos attributes.
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

	iRemovable, err := strconv.Atoi(removable)
	if err != nil {
		return false, errors.Wrapf(err, "removable output %q is not a number", removable)
	}

	return iRemovable == 1, nil
}
