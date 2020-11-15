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
	// Name of the storage device.
	Name string
	// Device contains the underlying hardware device type.
	Device Type
	// Failing contains a final assessment that the device failed or will fail soon.
	Status LifeStatus
	// PercentageUsed contains the percentage of SSD life that has been used.
	// For NVMe and SATA devices, an exact value is returned. For eMMC devices,
	// the value is reported in 10's of percents (10, 20, 30, etc.).
	// In case of any error reading SSD usage data, value will be -1.
	PercentageUsed int64
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
	var percentageUsed int64
	var name string
	switch deviceType {
	case EMMC:
		lifeStatus, err = parseDeviceHealtheMMC(lines)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse eMMC health")
		}
		name = parseDeviceNameEMMC(lines)
	case NVMe:
		percentageUsed, lifeStatus, err = parseDeviceHealthNVMe(lines)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse NVMe health")
		}
		name = parseDeviceNameNVMe(lines)
	case SSD:
		percentageUsed, lifeStatus, err = parseDeviceHealthSSD(lines)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse SSD health")
		}
		name = parseDeviceNameSATA(lines)
	default:
		return nil, errors.Errorf("parsing device health for type %v is not supported", deviceType)
	}

	return &Info{Name: name, Device: deviceType, Status: lifeStatus, PercentageUsed: percentageUsed}, nil
}

var (
	// nameDetectEMMC detects the name of a eMMC-based device using a regex.
	nameDetectEMMC = regexp.MustCompile(`\s*name\s+\|\s(?P<param>\S+).*`)
	// nameDetectNVMeSATA detects the name of a NVMe and SATA-based device using a regex.
	nameDetectNVMeSATA = regexp.MustCompile(`\s*Serial Number:\s+(?P<param>\S+).*`)
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
	// Example CSD text containing Pre EOL information. 0x03 means Urgent.
	//   "Pre EOL information [PRE_EOL_INFO: 0x03]"
	//     i.e. Urgent
	// We want to detect 0x03 for the Urgent case.
	// That indicates that the eMMC is near the end of life.
	emmcFailing = regexp.MustCompile(`.*(?P<param>PRE_EOL_INFO]?: 0x03)`)
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

// parseDeviceNameEMMC searches outlines for eMMC-based storage device name.
func parseDeviceNameEMMC(outLines []string) string {
	for _, line := range outLines {
		match := nameDetectEMMC.FindStringSubmatch(line)
		if match != nil {
			return match[1]
		}
	}

	return "EMMC"
}

// parseDeviceNameNVMe searches outlines for NVMe-based storage device name.
func parseDeviceNameNVMe(outLines []string) string {
	for _, line := range outLines {
		match := nameDetectNVMeSATA.FindStringSubmatch(line)
		if match != nil {
			return match[1]
		}
	}

	return "NVME"
}

// parseDeviceNameSATA searches outlines for SATA-based SSD storage device name.
func parseDeviceNameSATA(outLines []string) string {
	for _, line := range outLines {
		match := nameDetectNVMeSATA.FindStringSubmatch(line)
		if match != nil {
			return match[1]
		}
	}

	return "SATA"
}

// parseDeviceHealtheMMC analyzes eMMC for indications of failure. For additional information,
// refer to JEDEC standard 84-B50 which describes the extended CSD register. In this case,
// we focus on the PRE_EOL_INFO register.
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
func parsePercentageUsed(match []string) (int64, LifeStatus, error) {
	// Flag devices which report estimates approaching 100%

	percentageUsed, err := strconv.ParseInt(match[1], 10, 32)
	if err != nil {
		return 0, Healthy, errors.Errorf("failed to parse percentage used %v", match[1])
	}

	if percentageUsed > percentageUsedThreshold {
		return percentageUsed, Failing, nil
	}

	return percentageUsed, Healthy, nil
}

// parseDeviceHealthNVMe analyzes NVMe SMART attributes for indications of failure.
// Returns usage percentage, drive health status and error (if encountered).
func parseDeviceHealthNVMe(outLines []string) (int64, LifeStatus, error) {
	// Flag devices which report estimates approaching 100%

	for _, line := range outLines {
		match := nvmeFailing.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		return parsePercentageUsed(match)
	}

	return -1, Healthy, nil
}

// parseDeviceHealthSSD analyzes storage information for indications of failure specific to SSDs.
// Returns usage percentage, drive health status and error (if encountered).
func parseDeviceHealthSSD(outLines []string) (int64, LifeStatus, error) {
	// Flag devices which report estimates approaching 100% or that report failing
	// End-to-End_Error attribute

	for _, line := range outLines {
		if ssdFailingLegacy.MatchString(line) {
			return -1, Failing, nil
		}
		match := ssdFailing.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		return parsePercentageUsed(match)
	}

	return -1, Healthy, nil
}
