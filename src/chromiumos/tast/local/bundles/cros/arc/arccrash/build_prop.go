// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arccrash provides utilities for tests of crash reporting.
package arccrash

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

type buildProp struct {
	device      string
	board       string
	cpuAbi      string
	fingerprint string
}

func getProp(ctx context.Context, a *arc.ARC, key string) (string, error) {
	val, err := a.GetProp(ctx, key)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get %s", key)
	}
	if val == "" {
		return "", errors.Errorf("%s is empty", key)
	}
	return val, err
}

// GetBuildProp obtains the build property for ARC.
func GetBuildProp(ctx context.Context, a *arc.ARC) (*buildProp, error) {
	device, err := getProp(ctx, a, "ro.product.device")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device")
	}
	board, err := getProp(ctx, a, "ro.product.board")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get board")
	}
	cpuAbi, err := getProp(ctx, a, "ro.product.cpu.abi")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cpu_abi")
	}
	fingerprint, err := getProp(ctx, a, "ro.build.fingerprint")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get fingerprint")
	}

	return &buildProp{
		device:      device,
		board:       board,
		cpuAbi:      cpuAbi,
		fingerprint: fingerprint,
	}, nil
}

// UploadSystemBuildProp uploads /system/build.prop. This should be called to invetigate when GetPop failed. GetProp sometimes fails to get the device name
// even though the device name should always exists.
// See details in https://bugs.chromium.org/p/chromium/issues/detail?id=1039512#c16
func UploadSystemBuildProp(ctx context.Context, a *arc.ARC, outdir string) error {
	return a.PullFile(ctx, "/system/build.prop", filepath.Join(outdir, "build.prop"))
}

// ValidateBuildProp checks that given meta file for crash_sender contains the specified build properties.
func ValidateBuildProp(ctx context.Context, metafilePath string, bp *buildProp) (bool, error) {
	b, err := ioutil.ReadFile(metafilePath)
	if err != nil {
		return false, errors.Wrap(err, "failed to read meta file")
	}

	lines := strings.Split(string(b), "\n")
	contains := func(x string) bool {
		for _, l := range lines {
			if x == l {
				return true
			}
		}
		testing.ContextLogf(ctx, "Missing %q", x)
		return false
	}

	return contains("upload_var_device="+bp.device) &&
		contains("upload_var_board="+bp.board) &&
		contains("upload_var_cpu_abi="+bp.cpuAbi) &&
		contains("upload_var_arc_version="+bp.fingerprint), nil
}
