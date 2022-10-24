// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package version provides set of util functions used to work with ARC version properties.
package version

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

// BuildDescriptor contains essential parametrs of ARC Android image taken from test device.
type BuildDescriptor struct {
	// true in case built by ab/
	Official bool
	// ab/buildID
	BuildID string
	// build version in case build is official e.g 9138603
	BuildVersion int
	// build type e.g. user, userdebug
	BuildType string
	// Host ureadahead abi e.g. x86_64, arm, arm64
	HostUreadaheadAbi string
	// Guest cpu abi e.g. x86_64, x86, arm, arm64
	CPUAbi string
	// version release e.g. 9, 11
	VersionRelease int
	// ChromeOS milestone e.g 108
	Milestone int
}

func getHostUreadaheadAbi(ctx context.Context, dut *dut.DUT) (string, error) {
	b, err := dut.Conn().CommandContext(ctx, "file", "/sbin/ureadahead").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to check ureadahead remotely")
	}

	// Examples:
	// /sbin/ureadahead: ELF 64-bit LSB shared object, x86-64, version 1 (SYSV), dynamically linked, interpreter /lib64/ld-linux-x86-64.so.2, for GNU/Linux 3.2.0, BuildID[xxHash]=8cf6d9f23fc96e28, stripped
	// /sbin/ureadahead: ELF 32-bit LSB shared object, ARM, EABI5 version 1 (SYSV), dynamically linked, interpreter /lib/ld-linux-armhf.so.3, for GNU/Linux 3.2.0, BuildID[xxHash]=941ad6a55a036954, stripped
	// /sbin/ureadahead: ELF 64-bit LSB shared object, ARM aarch64, version 1 (SYSV), dynamically linked, interpreter /lib/ld-linux-aarch64.so.1, for GNU/Linux 3.7.0, BuildID[xxHash]=4daedd7720f6c1cf, stripped
	ureadaheadVersion := string(b)
	mVer := regexp.MustCompile(`^/sbin/ureadahead: ELF (32|64)-bit LSB shared object, (ARM aarch64|ARM|x86-64),.+\n`).FindStringSubmatch(ureadaheadVersion)
	if mVer == nil {
		return "", errors.Errorf("failed to parse ureadahead version: %q", ureadaheadVersion)
	}
	ureadaheadAbi := mVer[1] + " " + mVer[2]
	// Note, x86 is not expected and this is error condition.
	abiMap := map[string]string{
		"32 ARM":         "arm",
		"64 ARM aarch64": "arm64",
		"64 x86-64":      "x86_64",
	}

	abi, ok := abiMap[ureadaheadAbi]
	if !ok {
		return "", errors.Errorf("failed to map ureadahead architecture %q", ureadaheadAbi)
	}

	return abi, nil
}

// GetBuildDescriptorRemotely gets ARC build properties from the device, parses for build ID, ABI,
// and returns these fields as a combined string. It also return whether this is official build.
func GetBuildDescriptorRemotely(ctx context.Context, dut *dut.DUT, vmEnabled bool) (*BuildDescriptor, error) {
	var propertyFile string
	if vmEnabled {
		propertyFile = "/usr/share/arcvm/properties/build.prop"
	} else {
		propertyFile = "/usr/share/arc/properties/build.prop"
	}

	buildProp, err := dut.Conn().CommandContext(ctx, "cat", propertyFile).Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read ARC build property file remotely")
	}
	buildPropStr := string(buildProp)

	lsbRelease, err := dut.Conn().CommandContext(ctx, "cat", "/etc/lsb-release").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to lsb-release remotely")
	}
	lsbReleaseStr := string(lsbRelease)

	mCPUAbi := regexp.MustCompile(`(\n|^)ro.product.cpu.abi=(.+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mCPUAbi == nil {
		return nil, errors.Errorf("ro.product.cpu.abi is not found in %q", buildPropStr)
	}

	mBuildType := regexp.MustCompile(`(\n|^)ro.build.type=(.+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mBuildType == nil {
		return nil, errors.Errorf("ro.product.cpu.abi is not found in %q", buildPropStr)
	}

	// Note, this should work on official builds only. Custom built Android image contains the
	// version in different format.
	mBuildID := regexp.MustCompile(`(\n|^)ro.build.version.incremental=(.+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mBuildID == nil {
		return nil, errors.Errorf("ro.build.version.incremental is not found in %q", buildPropStr)
	}

	mVersionRelease := regexp.MustCompile(`(\n|^)ro.build.version.release=(\d+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mVersionRelease == nil {
		return nil, errors.Errorf("ro.build.version.release is not found in %q", buildPropStr)
	}

	versionRelease, err := strconv.Atoi(mVersionRelease[2])
	if err != nil {
		return nil, errors.Errorf("could not parse ro.build.version.release=%s: %q", mVersionRelease[2], err)
	}

	mMilestone := regexp.MustCompile(`(\n|^)CHROMEOS_RELEASE_CHROME_MILESTONE=(\d+)(\n|$)`).FindStringSubmatch(lsbReleaseStr)
	if mMilestone == nil {
		return nil, errors.Errorf("CHROMEOS_RELEASE_CHROME_MILESTONE is not found in %q", lsbReleaseStr)
	}

	milestone, err := strconv.Atoi(mMilestone[2])
	if err != nil {
		return nil, errors.Errorf("could not parse CHROMEOS_RELEASE_CHROME_MILESTONE=%s: %q", mMilestone[2], err)
	}

	buildVersion := 0
	official := regexp.MustCompile(`^\d+$`).MatchString(mBuildID[2])
	if official {
		buildVersion, err = strconv.Atoi(mBuildID[2])
		if err != nil {
			return nil, errors.Errorf("could not parse ro.build.version.incremental=%s: %q", mBuildID[2], err)
		}
	}

	abiMap := map[string]string{
		"armeabi-v7a": "arm",
		"arm64-v8a":   "arm64",
		"x86":         "x86",
		"x86_64":      "x86_64",
	}

	abi, ok := abiMap[mCPUAbi[2]]
	if !ok {
		return nil, errors.Errorf("failed to map ABI %q", mCPUAbi[2])
	}

	hostUreadaheadAbi, err := getHostUreadaheadAbi(ctx, dut)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get host ureadahead ABI")
	}

	desc := BuildDescriptor{
		Official:          official,
		BuildID:           mBuildID[2],
		BuildVersion:      buildVersion,
		BuildType:         mBuildType[2],
		HostUreadaheadAbi: hostUreadaheadAbi,
		CPUAbi:            abi,
		VersionRelease:    versionRelease,
		Milestone:         milestone,
	}

	return &desc, nil
}
