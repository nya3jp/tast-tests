// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"encoding/json"
	"os"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const (
	hardwareProbeBinary = "/usr/local/graphics/hardware_probe"
)

type pciDevice struct {
	BDF     string `json:'BDF'`
	Name    string `json:'Name'`
	BootVGA bool   `json:"BootVGA"`
}

type gpuInfo struct {
	Family string `json:'Family'`
	Vendor string `json:'GPUVendor'`
}

type hardwareProbeResult struct {
	VGADevice []pciDevice `json:"VGA_Devices"`
	GPUInfo   []gpuInfo   `json:"GPU_Family"`
	CPUFamily string      `json:"CPU_SOC_Family"`
}

// GetHardwareProbeResult saves the information to path and returns detailed information gathered by hardware_probe binaries in the DUT.
func GetHardwareProbeResult(ctx context.Context, path string) (hardwareProbeResult, error) {
	if out, err := testexec.CommandContext(ctx, hardwareProbeBinary, "-output="+path).Output(testexec.DumpLogOnError); err != nil {
		return hardwareProbeResult{}, errors.Wrapf(err, "failed to run %v, output: %v", hardwareProbeBinary, string(out))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return hardwareProbeResult{}, errors.Wrapf(err, "failed to read %v", path)
	}
	var result hardwareProbeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return hardwareProbeResult{}, errors.Wrapf(err, "failed to unmarshal data: %v", string(data))
	}
	return result, nil
}

func runAndGrepRegex(ctx context.Context, args []string, regexStr string) ([]string, error) {
	re := regexp.MustCompile(regexStr)
	out, err := testexec.CommandContext(ctx, hardwareProbeBinary, args...).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to run %v, output: %v", hardwareProbeBinary, string(out))
	}

	matches := re.FindAllStringSubmatch(string(out), -1)
	if matches == nil || len(matches) == 0 {
		return nil, errors.Errorf("regex %v fails on output %v", regexStr, string(out))
	}
	var result []string
	for _, match := range matches {
		result = append(result, match[1])
	}
	return result, nil
}

// CPUFamily returns the CPU SOC family on the host. e.g. intel, amd, qualcomm, etc.
func CPUFamily(ctx context.Context) (string, error) {
	match, err := runAndGrepRegex(ctx, []string{"--cpu-soc-family"}, `CPU_SOC_Family: (\w*)`)
	if err != nil {
		return "", err
	}
	return match[0], nil
}

// GPUFamilies returns the GPU families on the host. e.g. qualcomm, broadwell, kabylake, cezanne, etc.
func GPUFamilies(ctx context.Context) ([]string, error) {
	match, err := runAndGrepRegex(ctx, []string{"--gpu-family"}, `GPU_Family: (\w*)`)
	if err != nil {
		return nil, err
	}
	return match, nil
}
