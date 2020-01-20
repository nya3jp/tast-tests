// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package c2e2etest contains constants and utilities for the prebuilt android test APK.
package c2e2etest

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
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

// PullLogsAndCheckPassing pulls the specified gtest log files, checks the test passed, and returns the path of the pulled text log file.
func PullLogsAndCheckPassing(ctx context.Context, s *testing.State, arcFilePath string, localFilePrefix string, textLogName string, xmlLogName string) string {
	a := s.PreValue().(arc.PreData).ARC

	outLogFile := fmt.Sprintf("%s/%s%s", s.OutDir(), localFilePrefix, textLogName)
	outXMLFile := fmt.Sprintf("%s/%s%s", s.OutDir(), localFilePrefix, xmlLogName)

	if err := a.PullFile(ctx, arcFilePath+textLogName, outLogFile); err != nil {
		s.Fatalf("Failed to pull %s: %v", textLogName, err)
	}

	if err := a.PullFile(ctx, arcFilePath+xmlLogName, outXMLFile); err != nil {
		s.Fatalf("Failed to pull %s: %v", xmlLogName, err)
	}

	r, err := gtest.ParseReport(outXMLFile)
	if err != nil {
		s.Fatalf("Failed to parse XML file %s: %v", outXMLFile, err)
	}

	// Walk through the whole report and collect failed test cases and their messages.
	var failures []string
	space := regexp.MustCompile(`\s+`)
	for _, s := range r.Suites {
		for _, c := range s.Cases {
			if len(c.Failures) > 0 {
				// Report only the first error message as one line for each test case.
				msg := space.ReplaceAllString(c.Failures[0].Message, " ")
				failures = append(failures, fmt.Sprintf("\"%s.%s: %s\"", s.Name, c.Name, msg))
			}
		}
	}
	if failures != nil {
		s.Fatal("c2_e2e_test failed: ", failures)
	}

	return outLogFile
}
