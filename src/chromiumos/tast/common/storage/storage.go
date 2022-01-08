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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
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
	// TotalBytesWritten corresponds to total amount of data (in bytes) written
	// to the disk.
	TotalBytesWritten int64
}

// Get runs the storage info shell script and returns its info.
func Get(ctx context.Context) (*Info, error) {
	cmd := testexec.CommandContext(ctx, "sh", "-c", ". /usr/share/misc/storage-info-common.sh; get_storage_info")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run storage info command")
	}
	return parseGetStorageInfoOutput(ctx, out)
}

// parseGetStorageInfoOutput parses the storage information to find the device type and life status.
func parseGetStorageInfoOutput(ctx context.Context, out []byte) (*Info, error) {
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
	var percentageUsed, bytesWritten int64
	var name string
	switch deviceType {
	case EMMC:
		lifeStatus, err = parseDeviceHealtheMMC(lines)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse eMMC health")
		}
		name = parseDeviceNameEMMC(lines)
		percentageUsed, err = parsePercentageUsedEMMC(lines, emmcUsedA, emmcUsedB)
		if err != nil {
			testing.ContextLog(ctx, "Error acquiring usage of eMMC device: ", name, err)
		}
	case NVMe:
		lifeStatus, err = parseDeviceHealthNVMe(lines)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse NVMe health")
		}
		name = parseDeviceNameNVMe(lines)
		bytesWritten, err = parseTotalBytesWrittenNVMe(lines)
		if err != nil {
			testing.ContextLog(ctx, "Error acquiring TBW of NVMe device: ", name, err)
		}
		percentageUsed, err = parsePercentageUsed(lines, nvmeUsed)
		if err != nil {
			testing.ContextLog(ctx, "Error acquiring usage of NVMe device: ", name, err)
		}
	case SSD:
		lifeStatus, err = parseDeviceHealthSSD(lines)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse SSD health")
		}
		name = parseDeviceNameSATA(lines)
		bytesWritten, err = parseTotalBytesWrittenSATA(lines)
		if err != nil {
			testing.ContextLog(ctx, "Error acquiring TBW of SSD device: ", name, err)
		}
		percentageUsed, err = parsePercentageUsed(lines, ssdUsed)
		if err != nil {
			testing.ContextLog(ctx, "Error acquiring usage of SSD device: ", name, err)
		}
	default:
		return nil, errors.Errorf("parsing device health for type %v is not supported", deviceType)
	}

	return &Info{Name: name, Device: deviceType, Status: lifeStatus,
		PercentageUsed: percentageUsed, TotalBytesWritten: bytesWritten}, nil
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
	// emmcUsedA detects the Lifetime Estimation type A value for the eMMC drive.
	// Example eMMC usage text:
	// "Device life time estimation type A [DEVICE_LIFE_TIME_EST_TYP_A: 0x01]"
	emmcUsedA = regexp.MustCompile(`\s*.*DEVICE_LIFE_TIME_EST_TYP_A]?: 0x(?P<lifetime>0\S)`)
	// emmcUsedB detects the Lifetime Estimation type A value for the eMMC drive.
	// Example eMMC usage text:
	// "Device life time estimation type B [DEVICE_LIFE_TIME_EST_TYP_B: 0x01]"
	emmcUsedB = regexp.MustCompile(`\s*.*DEVICE_LIFE_TIME_EST_TYP_B]?: 0x(?P<lifetime>0\S)`)
	// nvmeUsed detects the usage (in percents) of the NVMe drive.
	// Example NVMe usage text: "	Percentage Used:                        0%"
	nvmeUsed = regexp.MustCompile(`\s*Percentage Used:\s*(?P<percentage>\d*)`)
	// nvmeSpare and nvmeThreshold are used to detect if nvme is failing using regex.
	// If Available Spare is less than Available Spare Threshold, the device
	// is likely close to failing and we should remove the DUT.
	// Example NVMe usage text: "	Available Spare:               100%"
	// "Available Spare Threshold:         10%"
	nvmeSpare     = regexp.MustCompile(`\s*Available Spare:\s+(?P<spare>\d+)%`)
	nvmeThreshold = regexp.MustCompile(`\s*Available Spare Threshold:\s+(?P<thresh>\d+)%`)
	// ssdUsed detects the usage of ssd device.
	// Example SSD usage text: "0x07  0x008  1              91  ---  Percentage Used Endurance Indicator"
	ssdUsed = regexp.MustCompile(`.*\s{3,}(?P<percentage>\d*).*Percentage Used Endurance Indicator`)
	// ssdFailingLegacy detects if ssd device is failing using a regex.
	// The indicator used here is not reported for all SATA devices.
	ssdFailingLegacy = regexp.MustCompile(`\s*(?P<param>\S+\s\S+)` + // ID and attribute name
		`\s+[P-][O-][S-][R-][C-][K-]` + // Flags
		`(\s+\d{3}){3}` + // Three 3-digit numbers
		`\s+NOW`) // Fail indicator
	// ssdFailing detects if ssd device is failing using a regex.
	// nvmeUnitsWritten is the regexp for matching TBW value for NVMe devices.
	nvmeUnitsWritten = regexp.MustCompile(`\s*Data Units Written:\s*(?P<param>\d+[,\d]*)`)
	// ssdUnitsWritten is the regexp for matching TBW value for SATA SSD devices.
	ssdUnitsWritten = regexp.MustCompile(`.*Total_LBAs_Written.*\s+(?P<param>\d+)$`)
	// We look for non-zero values for either attribute 160 Uncorrectable_Error_Cnt
	// or attribute 187 Reported_Uncorrect.
	// Example usage text: "187 Reported_Uncorrect      -O----   100   100   000    -    0"
	ssdFailing = regexp.MustCompile(`\s*(?P<param>(160\s+Uncorrectable_Error_Cnt|` +
		`187\s+Reported_Uncorrect))` + // ID and attribute name
		`\s+[P-][O-][S-][R-][C-][K-]` + // Flags
		`(\s+\d{1,3}){3}` + // Three 1 to 3-digit numbers
		`\s+(NOW|-)` + // Fail indicator
		`\s+(?P<value>[1-9][0-9]*)`) // Non-zero raw value
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

// parseDeviceHealthNVMe analyzes NVMe SMART attributes for indications of failure.
// Returns usage percentage, drive health status and error (if encountered).
func parseDeviceHealthNVMe(outLines []string) (LifeStatus, error) {
	// Flag devices which report available spare less than available threshold

	for i, line := range outLines {
		match := nvmeSpare.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		tmatch := nvmeThreshold.FindStringSubmatch(outLines[i+1])
		if tmatch == nil {
			return 0, errors.Errorf("failed to find available spare threshold %v", match[1])
		}

		sparePercent, err := strconv.ParseInt(match[1], 10, 32)
		if err != nil {
			return 0, errors.Errorf("failed to parse available spare %v", match[1])
		}

		threshPercent, err := strconv.ParseInt(tmatch[1], 10, 32)
		if err != nil {
			return 0, errors.Errorf("failed to parse available spare threshold %v", tmatch[1])
		}

		if sparePercent < threshPercent {
			return Failing, nil
		}
	}

	return Healthy, nil
}

// parseDeviceHealthSSD analyzes storage information for indications of failure specific to SSDs.
// Returns usage percentage, drive health status and error (if encountered).
func parseDeviceHealthSSD(outLines []string) (LifeStatus, error) {
	// Flag devices which report non-zero uncorrectable errors or that report failing
	// End-to-End_Error attribute

	for _, line := range outLines {
		if ssdFailingLegacy.MatchString(line) {
			return Failing, nil
		}
		match := ssdFailing.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		return Failing, nil

	}

	return Healthy, nil
}

// parsePercentageUsed is a helper function that analyzes the percentage used
// value for extracting disk usage.
func parsePercentageUsed(outLines []string, pattern *regexp.Regexp) (int64, error) {
	for _, line := range outLines {
		if match := pattern.FindStringSubmatch(line); match != nil {
			return strconv.ParseInt(match[1], 10, 32)
		}
	}

	return -1, nil
}

// parsePercentageUsedEMMC is a helper function that analyzes the lifetime estimation
// value for extracting disk usage. eMMC devices report two values for lifetime
// estimates, type A and type B, these values are determined by the vendor. These values
// also represent percentage ranges, for example, 0x01 indicates the device is
// 0% - 10% device life time used. To simplify our charts, we will just use the larger
// of the type A and type B values and convert the bucket value to an equivalent percentage which
// falls in the middle of the bucket the value represents. Each bucket represents a 10% range
// and we will take the median value of that range.
// So, the value 0x01 will be converted to 5% and the value 0x03 will be converted to 25%.
func parsePercentageUsedEMMC(outLines []string, patternA, patternB *regexp.Regexp) (int64, error) {
	var typeA, typeB, bucket int64
	var err error
	for _, line := range outLines {
		if match := patternA.FindStringSubmatch(line); match != nil {
			bucket, err = strconv.ParseInt(match[1], 16, 64)
			typeA = (bucket * 10) - 5
		}
		if match := patternB.FindStringSubmatch(line); match != nil {
			bucket, err = strconv.ParseInt(match[1], 16, 64)
			typeB = (bucket * 10) - 5
		}
	}
	if typeA != 0 || typeB != 0 {
		if typeA < typeB {
			return typeB, err
		}
		return typeA, err
	}

	return -1, err

}

// parseTotalBytesWrittenNVMe parses NVMe SMART attribute value to extract
// and return Total Bytes Written data.
func parseTotalBytesWrittenNVMe(lines []string) (int64, error) {
	for _, line := range lines {
		match := nvmeUnitsWritten.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		unitsWritten, err := strconv.ParseInt(strings.ReplaceAll(match[1], ",", ""), 10, 64)
		if err != nil {
			return 0, errors.Errorf("failed to parse total bytes written %v", match[1])
		}

		// smartctl reports units written in 1000's of blocks (512 bytes each).
		return unitsWritten * 512 * 1000, nil
	}

	return 0, nil
}

// parseTotalBytesWrittenSATA parses SATA SMART attribute value to extract
// and return Total Bytes Written data.
func parseTotalBytesWrittenSATA(lines []string) (int64, error) {
	for _, line := range lines {
		match := ssdUnitsWritten.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		blocksWritten, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return 0, errors.Errorf("failed to parse total bytes written %v", match[1])
		}

		return blocksWritten * 512, nil
	}

	return 0, nil
}
