// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// BootMode is enum of the possible DUT states (besides OFF).
type BootMode int

// DUTs have three possible boot modes: Normal, Dev, and Recovery.
const (
	BootModeNormal   BootMode = iota
	BootModeDev      BootMode = iota
	BootModeRecovery BootMode = iota
)

// rePartition finds the partition number at the end of a device name.
var rePartition = regexp.MustCompile("p?[0-9]+$")

// CheckCrossystemValues calls crossystem to check whether the specified key-value pairs are present.
// We use the following crossystem syntax, which returns an error code of 0
// if (and only if) all key-value pairs match:
//     crossystem param1?value1 [param2?value2 [...]]
func CheckCrossystemValues(ctx context.Context, values map[string]string) bool {
	cmdArgs := make([]string, len(values))
	i := 0
	for k, v := range values {
		cmdArgs[i] = fmt.Sprintf("%s?%s", k, v)
		i++
	}
	_, err := testexec.CommandContext(ctx, "crossystem", cmdArgs...).Output(testexec.DumpLogOnError)
	return err == nil
}

// CheckBootMode determines whether the DUT is in the specified boot mode based on crossystem values.
func CheckBootMode(ctx context.Context, mode BootMode) (bool, error) {
	var crossystemValues map[string]string
	switch mode {
	case BootModeNormal:
		crossystemValues = map[string]string{"devsw_boot": "0", "mainfw_type": "normal"}
	case BootModeDev:
		crossystemValues = map[string]string{"devsw_boot": "1", "mainfw_type": "developer"}
	case BootModeRecovery:
		crossystemValues = map[string]string{"mainfw_type": "recovery"}
	default:
		return false, errors.Errorf("unrecognized boot mode %d", mode)
	}
	return CheckCrossystemValues(ctx, crossystemValues), nil
}

// RootDevice finds the name of the root device, strips off the partition number, and returns it.
// Sample output: '/dev/mmcblk1' (having stripped the partition number from '/dev/mmcblk1p3')
func RootDevice(ctx context.Context) (string, error) {
	deviceWithPart, err := testexec.CommandContext(ctx, "rootdev", "-s").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	deviceWithPartStr := strings.TrimSpace(string(deviceWithPart))
	return rePartition.ReplaceAllString(deviceWithPartStr, ""), nil
}

// BootDeviceRemovable checks whether the current boot device is removable.
func BootDeviceRemovable(ctx context.Context) (bool, error) {
	rootDevice, err := RootDevice(ctx)
	if err != nil {
		return false, err
	}
	return deviceRemovable(ctx, rootDevice)
}

// deviceRemovable checks whether a certain storage device is removable.
func deviceRemovable(ctx context.Context, device string) (bool, error) {
	fp := fmt.Sprintf("/sys/block/%s/removable", filepath.Base(device))
	content, err := ioutil.ReadFile(fp)
	if err != nil {
		return false, errors.Wrapf(err, "reading filepath %s", fp)
	}
	removable, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return false, err
	}
	return removable == 1, nil
}

// internalDisk determines the internal disk based on the current disk.
// If device is a removable device, then the internal disk is determined
// based on the type of device (arm or x86).
// Otherwise, return device itself.
func internalDisk(ctx context.Context, device string) (string, error) {
	removable, err := deviceRemovable(ctx, device)
	if err != nil {
		return "", err
	}
	if removable {
		for _, p := range []string{"/dev/mmcblk0", "/dev/mmcblk1", "/dev/nvme0n1"} {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				return p, nil
			}
		}
		return "/dev/sda", nil
	}
	return device, nil
}

// InternalDevice returns the internal disk based on the current disk.
func InternalDevice(ctx context.Context) (string, error) {
	rootDevice, err := RootDevice(ctx)
	if err != nil {
		return "", err
	}
	disk, err := internalDisk(ctx, rootDevice)
	if err != nil {
		return "", err
	}
	return disk, nil
}
