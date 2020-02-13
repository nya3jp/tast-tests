// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package storage reports information retrieved from storage-info-common.sh on behalf of tests.
package storage

import (
	"bytes"
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// Type stands for various Chromebook storage devices.
type Type int

// LifeStatus stands for a simplified overview of device health.
type LifeStatus int

const (
	// EMMC (Embedded Multi-Media Controller) devices are a single package flash storage and controller.
	EMMC Type = iota
	// NVMe (Non-Volatile Memory Express) interface. PCIe cards, but more commonly M.2 in Chromebooks.
	NVMe
	// SSD (Solid State Drive) devices connected through a SATA interface.
	SSD
)

const (
	// Healthy means that the device does not indicate failure or limited remaining life time.
	Healthy LifeStatus = iota
	// Failing indicates the storage device failed or will soon.
	Failing
)

// Info contains information about a storage device.
type Info struct {
	// Device contains the underlying hardware device type.
	Device Type
	// Failing contains a final assessment that the device failed or will fail soon.
	Status LifeStatus
}

// Get runs the storage info shell script and returns its info.
func Get(ctx context.Context) (*Info, error) {
	cmd := testexec.CommandContext(ctx, "sh", "-c", ". /usr/share/misc/storage-info-common.sh; get_storage_info")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run storage info command")
	}
	return parseGetStorageInfoOutput(out)
}

// parseGetStorageInfoOutput parses the storage information to find the device type and life status.
func parseGetStorageInfoOutput(out []byte) (*Info, error) {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return nil, errors.New("get storage info did not produce output")
	}

	lines := strings.Split(string(out), "\n")

	deviceType, err := parseDeviceType(lines)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse storage info for device type")
	}

	var lifeStatus LifeStatus
	switch deviceType {
	case EMMC:
		lifeStatus, err = parseDeviceHealtheMMC(lines)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse eMMC health")
		}
	case NVMe:
		lifeStatus, err = parseDeviceHealthNVMe(lines)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse NVMe health")
		}
	case SSD:
		lifeStatus, err = parseDeviceHealthSSD(lines)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse SSD health")
		}
	default:
		return nil, errors.Errorf("parsing device health for type %v is not supported", deviceType)
	}

	return &Info{Device: deviceType, Status: lifeStatus}, nil
}

var (
	// nvmeDetect detects if storage device is NVME using a regex.
	// Example NVMe SMART text: "   SMART/Health Information (NVMe Log 0x02, NSID 0xffffffff)"
	nvmeDetect = regexp.MustCompile(`\s*SMART.*NVMe Log`)
	// ssdDetect detects if storage device is SSD using a regex.
	// Example SSD ATA text, "      ATA Version is:   ACS-2 T13/2015-D revision 3"
	ssdDetect = regexp.MustCompile(`\s*ATA Version`)
	// emmcDetect detects if storage device is eMMC using a regex.
	// Example eMMC CSD text, "  Extended CSD rev 1.8 (MMC 5.1)"
	emmcDetect = regexp.MustCompile(`\s*Extended CSD rev.*MMC`)

	// emmcVersion finds eMMC version of device using a regex.
	// Example eMMC CSD text, "  Extended CSD rev 1.8 (MMC 5.1)".
	emmcVersion = regexp.MustCompile(`\s*Extended CSD rev.*MMC (?P<version>\d+.\d+)`)
	// emmcFailing detects if eMMC device is failing using a regex.
	// Example CSD text containing life time registers. 0x0a means 90-100% band,
	// 0x0b means over 100% band. Find none digits.
	//   "Device life time estimation type B [DEVICE_LIFE_TIME_EST_TYP_B: 0x01]
	//     i.e. 0% - 10% device life time used
	//    Device life time estimation type A [DEVICE_LIFE_TIME_EST_TYP_A: 0x00]"
	// If the last value is not a digit, it means we found 0x0a or 0x0b, indicating
	// over 90% life, our failure case.
	emmcFailing = regexp.MustCompile(`.*(?P<param>DEVICE_LIFE_TIME_EST_TYP_.)]?: 0x0\D`)
	// nvmeFailing detects if nvme is failing using a regex.
	// Example NVMe usage text: "	Percentage Used:                        0%"
	nvmeFailing = regexp.MustCompile(`\s*Percentage Used:\s*(?P<percentage>\d*)`)
	// ssdFailingLegacy detects if ssd device is failing using a regex.
	// The indicator used here is not reported for all SATA devices.
	ssdFailingLegacy = regexp.MustCompile(`\s*(?P<param>\S+\s\S+)` + // ID and attribute name
		`\s+[P-][O-][S-][R-][C-][K-]` + // Flags
		`(\s+\d{3}){3}` + // Three 3-digit numbers
		`\s+NOW`) // Fail indicator
	// ssdFailing detects if ssd device is failing using a regex.
	// Example SSD usage text: "0x07  0x008  1              91  ---  Percentage Used Endurance Indicator"
	ssdFailing = regexp.MustCompile(`.*\s{3,}(?P<percentage>\d*).*Percentage Used Endurance Indicator`)

	// percentageUsedThreshold is the threshold for percentage used values we
	// flag as indicating a failing device. If an NVMe or SATA device indicates
	// a percentage used value above this threshold, we flag the device as failing.
	percentageUsedThreshold int64 = 97
)

// parseDeviceType searches outlines for storage device type.
func parseDeviceType(outLines []string) (Type, error) {
	for _, line := range outLines {
		if nvmeDetect.MatchString(line) {
			return NVMe, nil
		}

		if ssdDetect.MatchString(line) {
			return SSD, nil
		}

		if emmcDetect.MatchString(line) {
			return EMMC, nil
		}
	}

	return 0, errors.New("failed to detect a device type")
}

// parseDeviceHealtheMMC analyzes eMMC for indications of failure. For additional information,
// refer to JEDEC standard 84-B50 which describes the extended CSD register. In this case,
// we focus on DEVICE_LIFE_TIME_EST_TYPE_B and TYPE_A registers.
func parseDeviceHealtheMMC(outLines []string) (LifeStatus, error) {
	// Device life estimates were introduced in version 5.0
	const emmcMinimumVersion = 5.0

	for _, line := range outLines {
		match := emmcVersion.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		version, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return 0, errors.Errorf("failed to parse eMMC version %v", match[1])
		}

		if version < emmcMinimumVersion {
			return 0, errors.Errorf("eMMC version %v less than %v", version, emmcMinimumVersion)
		}
	}

	for _, line := range outLines {
		if emmcFailing.MatchString(line) {
			return Failing, nil
		}
	}

	return Healthy, nil
}

// parsePercentageUsed is a helper function that analyzes the percentage used
// value for indications of a failure.
func parsePercentageUsed(match []string) (LifeStatus, error) {
	// Flag devices which report estimates approaching 100%

	percentageUsed, err := strconv.ParseInt(match[1], 10, 32)
	if err != nil {
		return 0, errors.Errorf("failed to parse percentage used %v", match[1])
	}

	if percentageUsed > percentageUsedThreshold {
		return Failing, nil
	}

	return Healthy, nil
}

// parseDeviceHealthNVMe analyzes NVMe SMART attributes for indications of failure.
func parseDeviceHealthNVMe(outLines []string) (LifeStatus, error) {
	// Flag devices which report estimates approaching 100%

	for _, line := range outLines {
		match := nvmeFailing.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		return parsePercentageUsed(match)
	}

	return Healthy, nil
}

// parseDeviceHealthSSD analyzes storage information for indications of failure specific to SSDs.
func parseDeviceHealthSSD(outLines []string) (LifeStatus, error) {
	// Flag devices which report estimates approaching 100% or that report failing
	// End-to-End_Error attribute

	for _, line := range outLines {
		if ssdFailingLegacy.MatchString(line) {
			return Failing, nil
		}
		match := ssdFailing.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		return parsePercentageUsed(match)
	}

	return Healthy, nil
}
