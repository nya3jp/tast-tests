// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const (
	hardwareProbeBinary = "/usr/local/graphics/hardware_probe"
)

func runAndGrepRegex(ctx context.Context, args []string, regexStr string) ([]string, error) {
	re := regexp.MustCompile(regexStr)
	out, err := testexec.CommandContext(ctx, hardwareProbeBinary, args...).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to run %v", hardwareProbeBinary)
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
