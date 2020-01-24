// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"fmt"
	"io/ioutil"
	"os"
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
	cmd := testexec.CommandContext(ctx, "crossystem", cmdArgs...)
	_, err := cmd.Output(testexec.DumpLogOnError)
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

// pathExists determines whether a filepath represents a valid file/dir.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// GetRootPart returns the name of the root device with the partition number.
func GetRootPart(ctx context.Context) (string, error) {
	cmd := testexec.CommandContext(ctx, "rootdev", "-s")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetRootDevice returns the name of the root device without the partition number.
func GetRootDevice(ctx context.Context) (string, error) {
	rootPart, err := GetRootPart(ctx)
	if err != nil {
		return "", err
	}
	rootDev := stripPart(rootPart)
	return rootDev, nil
}

// stripPart takes the name of the root device with the partition number, and removes the partition number.
func stripPart(rootPart string) string {
	rePart := regexp.MustCompile("p?[0-9]+$")
	return rePart.ReplaceAllString(rootPart, "")
}

// IsBootDeviceRemovable checks whether the current boot device is removable.
func IsBootDeviceRemovable(ctx context.Context) (bool, error) {
	rootPart, err := GetRootPart(ctx)
	if err != nil {
		return false, err
	}
	return IsDeviceRemovable(ctx, rootPart)
}

// IsDeviceRemovable checks whether a certain storage device is removable.
func IsDeviceRemovable(ctx context.Context, devicePart string) (bool, error) {
	device := stripPart(devicePart)
	deviceBasename := strings.Split(device, "/")[2]
	fp := fmt.Sprintf("/sys/block/%s/removable", deviceBasename)
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

// GetInternalDisk determines the internal disk based on the current disk.
// If devicePart is a removable device, then the internal disk is determined
// based on the type of device (arm or x86).
// Otherwise, return device itself.
func GetInternalDisk(ctx context.Context, devicePart string) (string, error) {
	if isRemovable, err := IsDeviceRemovable(ctx, devicePart); err != nil {
		return "", err
	} else if isRemovable {
		for _, p := range []string{"/dev/mmcblk0", "/dev/mmcblk1", "/dev/nvme0n1"} {
			if pathExists(p) {
				return p, nil
			}
		}
		return "/dev/sda", nil
	} else {
		return stripPart(devicePart), nil
	}
}

// GetInternalDevice returns the internal disk based on the current disk.
func GetInternalDevice(ctx context.Context) (string, error) {
	rootPart, err := GetRootPart(ctx)
	if err != nil {
		return "", err
	}
	internalDisk, err := GetInternalDisk(ctx, rootPart)
	if err != nil {
		return "", err
	}
	return internalDisk, nil
}
