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

// BootedDeviceType is the set of mode,device,kernel verifier for the DUT
type BootedDeviceType string

// The set of all possible BootedDeviceTypes, add more as needed
const (
	BootedDeviceTypeNormalInternalSig      BootedDeviceType = "normal mode booted from internal device, signature verified kernel key"
	BootedDeviceTypeDeveloperInternalSig   BootedDeviceType = "developer mode booted from internal device, signature verified kernel key"
	BootedDeviceTypeDeveloperRemovableSig  BootedDeviceType = "developer mode booted from removable device, signature verified kernel key"
	BootedDeviceTypeDeveloperRemovableHash BootedDeviceType = "developer mode booted from removable device, hash verified kernel key"
)

type lookupTuple struct {
	mode      string
	removable bool
	vfy       string
}

var verificationMap = map[lookupTuple]BootedDeviceType{
	lookupTuple{"normal", false, "sig"}:    BootedDeviceTypeNormalInternalSig,
	lookupTuple{"developer", false, "sig"}: BootedDeviceTypeDeveloperInternalSig,
	lookupTuple{"developer", true, "sig"}:  BootedDeviceTypeDeveloperRemovableSig,
	lookupTuple{"developer", true, "hash"}: BootedDeviceTypeDeveloperRemovableHash,
}

const (
	targetHostedFile string = "/etc/lsb-release"
)

var rTargetHosted = regexp.MustCompile(`(?i)chrom(ium|e)os`)
var rDevNameStripper = regexp.MustCompile(`p?[0-9]+$`)

// Report the current BootedDeviceType of the dut
func (r *reporter) BootedDevice(ctx context.Context) (BootedDeviceType, error) {
	if cs, err := r.Crossystem(ctx, CrossystemTypeMainfwType, CrossystemTypeKernvfyKey); err != nil {
		return "", err
	} else if rootPart, err := getRootPartition(ctx, r); err != nil {
		return "", errors.Wrap(err, "failed to get root partition")
	} else if removable, err := isRemovableDevice(ctx, r, rootPart); err != nil {
		return "", errors.Wrapf(err, "failed to determine if %q is removable", rootPart)
	} else if bootedDevice, found := verificationMap[lookupTuple{cs[CrossystemTypeMainfwType], removable, cs[CrossystemTypeKernvfyKey]}]; !found {
		return "", errors.Errorf("unrecognized combination of %v=%v, %v=%v, %v=%v", CrossystemTypeMainfwType, cs[CrossystemTypeMainfwType], "removable", removable, CrossystemTypeKernvfyKey, cs[CrossystemTypeKernvfyKey])
	} else {
		return bootedDevice, nil
	}
}

// getRootPartition gets the root partition as reported by the 'rootdev -s' command
func getRootPartition(ctx context.Context, r *reporter) (string, error) {
	if res, err := r.CommandLines(ctx, "rootdev", "-s"); err != nil {
		return "", errors.Wrap(err, "failed to determine root partition")
	} else if len(res) == 0 || res[0] == "" {
		return "", errors.New("invalid empty string root partition")
	} else {
		return res[0], nil
	}
}

// isTargetHosted determines if DUT is hosted by checking if /etc/lsb-release has chromiumos attributes
func isTargetHosted(ctx context.Context, r *reporter) (bool, error) {
	if res, err := r.CatFileLines(ctx, targetHostedFile); err != nil {
		return false, err
	} else if len(res) == 0 {
		return false, nil
	} else {
		return rTargetHosted.FindStringIndex(res[0]) != nil, nil
	}
}

// isRemovableDevice determines if DUT is hosted by checking if /etc/lsb-release has chromiumos attributes
func isRemovableDevice(ctx context.Context, r *reporter, device string) (bool, error) {
	if res, err := isTargetHosted(ctx, r); err != nil {
		return false, err
	} else if !res {
		return false, nil
	}

	// removes the partition portion of the device
	baseDev := rDevNameStripper.ReplaceAllString(strings.Split(device, "/")[2], "")
	if res, err := r.CatFile(ctx, fmt.Sprintf("/sys/block/%s/removable", baseDev)); err != nil {
		return false, err
	} else if i, err := strconv.Atoi(res); err != nil {
		return false, errors.Wrapf(err, "removable output %q is not a number", res)
	} else {
		return i == 1, nil
	}
}
