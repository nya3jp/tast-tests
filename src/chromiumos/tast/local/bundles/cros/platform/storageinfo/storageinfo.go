// Copyright 2019 The Chromium OS Authors.i All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package storageinfo reports information retrieved from storage-info-common.sh on behalf of tests.
package storageinfo

import (
	"bytes"
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// StorageDevice stands for various Chromebook storage devices.
type StorageDevice int

// DeviceLifeStatus stands for a simplified overview of device health.
type DeviceLifeStatus int

const (
	// eMMC (Embedded Multi-Media Controller) devices are a single package flash storage and controller.
	eMMC StorageDevice = iota
	// NVMe (Non-Volatile Memory Express) interface. PCIe cards, but more commonly M.2 in Chromebooks.
	NVMe
	// SSD (Solid State Drive) devices connected through a SATA interface.
	SSD
	// StorageDeviceUnknown represents any device which does fit the above categories.
	StorageDeviceUnknown
)

const (
	// Healthy means that the device does not indicate failure or limited remaining life time.
	Healthy DeviceLifeStatus = iota
	// Failing indicates the storage device failed or will soon.
	Failing
	// NotSupported means the device (such as older eMMC versions) does not report health information.
	NotSupported
)

// Info contains information about a storage device.
type Info struct {
	// Type contains the underlying hardware device type.
	Type StorageDevice
	// Failing contains a final assessment that the device failed or will fail soon.
	Status DeviceLifeStatus
}

// Get runs the storage info shell script and returns its info.
func Get(ctx context.Context) (*Info, error) {
	cmd := testexec.CommandContext(ctx, "sh", "-c", ". /usr/share/misc/storage-info-common.sh; get_storage_info")
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return nil, errors.Wrap(err, "Failed to run storage info command")
	}
	return parseGetStorageInfoOutput(out)
}

// parseGetStorageInfoOutput parses the storage information to find the device type and life status.
func parseGetStorageInfoOutput(out []byte) (*Info, error) {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return nil, errors.New("Get storage info did not produce output")
	}

	lines := strings.Split(string(out), "\n")

	deviceType, err := parseDeviceType(lines)
	if err != nil {
		return nil, errors.New("Failed to parse storage info for device type")
	}

	var deviceLifeStatus DeviceLifeStatus
	switch deviceType {
	case eMMC:
		deviceLifeStatus, err = parseDeviceHealtheMMC(lines)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse eMMC health")
		}
	case NVMe:
		deviceLifeStatus, err = parseDeviceHealthNVMe(lines)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse NVMe health")
		}
	case SSD:
		deviceLifeStatus = parseDeviceHealthSSD(lines)
	default:
		return nil, errors.Errorf("Parsing device health for type %v is not supported", deviceType)
	}

	info := &Info{
		Type:   deviceType,
		Status: deviceLifeStatus,
	}
	return info, nil
}

// praseDeviceType searches outlines for storage device type.
func parseDeviceType(outLines []string) (StorageDevice, error) {
	// Example NVMe SMART text: "	SMART/Health Information (NVMe Log 0x02, NSID 0xffffffff)"
	nvmeDetect := regexp.MustCompile(`\s*SMART.*NVMe Log`)
	// Example SSD ATA text, "	ATA Version is:   ACS-2 T13/2015-D revision 3"
	ssdDetect := regexp.MustCompile(`\s*ATA Version`)
	// Example eMMC CSD text, "  Extended CSD rev 1.8 (MMC 5.1)"
	emmcDetect := regexp.MustCompile(`\s*Extended CSD rev.*MMC`)

	for _, line := range outLines {
		if nvmeDetect.MatchString(line) {
			return NVMe, nil
		}

		if ssdDetect.MatchString(line) {
			return SSD, nil
		}

		if emmcDetect.MatchString(line) {
			return eMMC, nil
		}
	}

	return StorageDeviceUnknown, errors.New("Failed to detect a device type")
}

// parseDeviceHealtheMMC analyzes eMMC for indications of failure. For additional information,
// refer to JEDEC standard 84-B50 which describes the extended CSD register. In this case,
// we focus on DEVICE_LIFE_TIME_EST_TYPE_B and TYPE_A registers.
func parseDeviceHealtheMMC(outLines []string) (DeviceLifeStatus, error) {
	// Device life estimates were introduced in version 5.0
	const eMMCMinimumVersion = 5.0
	// Example eMMC CSD text, "  Extended CSD rev 1.8 (MMC 5.1)".
	eMMCVersion := regexp.MustCompile(`\s*Extended CSD rev.*MMC (?P<version>\d+.\d+)`)
	// Example CSD text containing life time registers. 0x0a means 90-100% band,
	// 0x0b means over 100% band. Find none digits.
	//   "Device life time estimation type B [DEVICE_LIFE_TIME_EST_TYP_B: 0x01]
	//     i.e. 0% - 10% device life time used
	//    Device life time estimation type A [DEVICE_LIFE_TIME_EST_TYP_A: 0x00]"
	eMMCFailing := regexp.MustCompile(`.*(?P<param>DEVICE_LIFE_TIME_EST_TYP_.): 0x0\D`)

	for _, line := range outLines {
		match := eMMCVersion.FindAllStringSubmatch(line, -1)
		if match != nil {
			version, err := strconv.ParseFloat(match[0][1], 64)
			if err != nil {
				return NotSupported, errors.New("Failed to parse eMMC version")
			}
			if version < 5.0 {
				return NotSupported, nil
			}
		}
	}

	for _, line := range outLines {
		if eMMCFailing.MatchString(line) {
			return Failing, nil
		}
	}

	return Healthy, nil
}

// parseDeviceHealthNVMe analyzes NVMe SMART attributes for indications of failure.
func parseDeviceHealthNVMe(outLines []string) (DeviceLifeStatus, error) {
	// Flag devices which report estimates approaching 100%
	const percentageUsedThreshold = 97
	// Example NVMe usage text: "	Percentage Used:                        0%"
	nvmeFailing := regexp.MustCompile(`\s*Percentage Used:\s*(?P<percentage>\d*)`)

	for _, line := range outLines {
		match := nvmeFailing.FindAllStringSubmatch(line, -1)
		if match != nil {
			percentageUsed, err := strconv.ParseInt(match[0][1], 10, 32)
			if err != nil {
				return NotSupported, errors.New("Failed to parse NVMe percentage used")
			}

			if percentageUsed > percentageUsedThreshold {
				return Failing, nil
			}
		}
	}

	return Healthy, nil
}

// parseDeviceHealthSSD analyzes storage information for indications of failure specific to SSDs.
func parseDeviceHealthSSD(outLines []string) DeviceLifeStatus {
	ssdFail := `\s*(?P<param>\S+\s\S+)` + // ID and attribute name
		`\s+[P-][O-][S-][R-][C-][K-]` + // Flags
		`(\s+\d{3}){3}` + // Three 3-digit numbers
		`\s+NOW` // Fail indicator
	ssdFailing := regexp.MustCompile(ssdFail)

	for _, line := range outLines {
		if ssdFailing.MatchString(line) {
			return Failing
		}
	}

	return Healthy
}
