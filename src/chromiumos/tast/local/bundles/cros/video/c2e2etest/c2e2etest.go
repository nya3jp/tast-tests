// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package c2e2etest contains constants and utilities for the prebuilt android test APK.
package c2e2etest

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
)

const (
	// Pkg is the package name of the test APK
	Pkg = "org.chromium.c2.test"
	// ActivityName is the name of the test activity
	ActivityName = ".E2eTestActivity"

	// X86ApkName is the name of the c2_e2e_test apk for x86/x86_64 devices
	X86ApkName = "c2_e2e_test_x86.apk"
	// ArmApkName is the name of the c2_e2e_test apk for arm devices
	ArmApkName = "c2_e2e_test_arm.apk"
)

// GetApkNameForArch gets the name of the APK file to install on the DUT.
func GetApkNameForArch(ctx context.Context, a *arc.ARC) (string, error) {
	out, err := a.Command(ctx, "getprop", "ro.product.cpu.abi").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get abi: %v", err)
	}

	if strings.HasPrefix(string(out), "x86") {
		return X86ApkName, nil
	}
	return ArmApkName, nil
}

// GrantApkPermissions grants the permissions necessary for the test APK.
func GrantApkPermissions(ctx context.Context, a *arc.ARC) error {
	permissions := [2]string{
		"android.permission.READ_EXTERNAL_STORAGE",
		"android.permission.WRITE_EXTERNAL_STORAGE"}
	for _, perm := range permissions {
		if err := a.Command(ctx, "pm", "grant", Pkg, perm).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to grant permission: %v", err)
		}
	}
	return nil
}
