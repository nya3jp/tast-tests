// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"math"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// QualParam is the configuration of dual-qual functionality.
type QualParam struct {
	IsSlcEnabled           bool
	SlcDevice              string
	TestDevice             string
	RetentionBlockTimeout  time.Duration
	SuspendBlockTimeout    time.Duration
	SkipS0iXResidencyCheck bool
}

// subTestFunc is the code associated with a sub-test.
type subTestFunc func(context.Context, *testing.State, *FioResultWriter, QualParam)

// Swapoff disables swap.
func Swapoff(ctx context.Context) error {
	testing.ContextLog(ctx, "Disabling swap")
	err := testexec.CommandContext(ctx, "swapoff", "-a").Run(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to turn swap off")
	}

	return nil
}

// SetupChecks verifys the size of the storage device matches the
// user requested size.
func SetupChecks(ctx context.Context, s *testing.State) {
	// Fetching info of all storage devices.
	info, err := ReadDiskInfo(ctx)
	if err != nil {
		s.Fatal("Failed reading disk info: ", err)
	}

	// Checking the size of the main storage device.
	if err := info.CheckMainDeviceSize(mainStorageDeviceMinSize); err != nil {
		s.Fatal("Main storage disk is too small: ", err)
	}

	// Save storage info to results.
	if err := info.SaveDiskInfo(filepath.Join(s.OutDir(), "diskinfo.json")); err != nil {
		s.Fatal("Error saving disk info: ", err)
	}

	// Get the user-requested and actual disk size.
	varStr := s.RequiredVar("tast_disk_size_gb")
	requestedSizeGb, err := strconv.Atoi(varStr)
	if err != nil {
		s.Fatal("Bad format of request disk size: ", err)
	}

	actualSizeGb, err := info.SizeInGB()
	if err != nil {
		s.Fatal("Error selecting main storage device: ", err)
	}

	// Check if the requested disk size is within 10% of the actual.
	// Threshold is needed because we want to treat 512GB and 500GB as the same size.
	if int(math.Abs(float64(actualSizeGb-requestedSizeGb))) > actualSizeGb/10 {
		s.Fatalf("Requested disk size %dGB doesn't correspond to to the actual size %dGB",
			requestedSizeGb, actualSizeGb)
	}
}

// SlcDevice returns an Slc device path for dual-namespace AVL.
func SlcDevice(ctx context.Context) (string, error) {
	info, err := ReadDiskInfo(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed reading disk info")
	}
	slc, err := info.SlcDevice()
	if slc == nil {
		return "", errors.Wrap(err, "dual qual is specified but SLC device is not present")
	}
	return filepath.Join("/dev/", slc.Name), nil
}

// RemovableDevice returns a removable device for external storage AVL
func RemovableDevice(ctx context.Context) (string, error) {
	info, err := ReadDiskInfo(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed reading disk info")
	}

	removable, err := info.RemovableDevice(ctx)
	if removable == nil {
		return "", errors.Wrap(err, "removable qual is specified but removable device is not present")
	}
	if err != nil {
		return "", errors.Wrap(err, "failed to set removable device")
	}

	testing.ContextLog(ctx, "Removable device: ", removable.Name)
	dev := filepath.Join("/dev/", removable.Name)

	// If the device is partitioned, use partition 1 to preserve the partition table.
	partitionDevName := AppendPartition(dev, "1")
	cmd := testexec.CommandContext(ctx, "fdisk", "-l", partitionDevName)
	_, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLog(ctx, "Partition not found, using device ", dev)
		return dev, nil
	}
	testing.ContextLog(ctx, "Partition found, using device ", partitionDevName)
	return partitionDevName, nil

}
