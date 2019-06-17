// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	coverageDataDir  = "coverage_data/profraws"
	coveredBinaryDir = "coverage_data/binaries"
)

var (
	daemonNamesForCoverage = []string{"tpm_managerd", "attestationd", "cryptohomed"}
	binaryNamesForCoverage = []string{"tpm_managerd", "attestationd", "cryptohomed", "local_data_migration"}
)

func flushCoverageData(ctx context.Context) error {
	if !isRemoteTest(ctx) {
		return errors.New("reboot operation only supported in remote test")
	}
	os.MkdirAll(coverageDataDir, 0777)
	os.MkdirAll(coveredBinaryDir, 0777)
	d, ok := dut.FromContext(ctx)
	if !ok {
		errors.New("Failed to get DUT")
	}
	for _, daemon := range daemonNamesForCoverage {
		cmd := "restart " + daemon
		if _, err := d.Command("sh", "-c", cmd).CombinedOutput(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to restart "+cmd)
			continue
		}
	}
	if err := collectProfraws(ctx); err != nil {
		return errors.Wrap(err, "failed to collect profraw files")
	}
	return nil
}

func collectProfraws(ctx context.Context) error {
	if !isRemoteTest(ctx) {
		return errors.New("reboot operation only supported in remote test")
	}
	d, ok := dut.FromContext(ctx)
	if !ok {
		errors.New("Failed to get DUT")
	}
	cmd := "find /tmp -path /tmp*profraw"
	result, err := d.Command("sh", "-c", cmd).CombinedOutput(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to check if any profraws in /tmp")
	}
	hasCollected := false
	for _, filepath := range strings.Fields(string(result)) {
		cmd := "cat " + string(filepath)
		testing.ContextLog(ctx, "Running "+cmd)
		result, err := d.Command("sh", "-c", cmd).CombinedOutput(ctx)
		// Don't stop just because of failure of a transfer
		if err != nil {
			testing.ContextLog(ctx, "Failed to run "+cmd)
			continue
		}
		// Writes the content to the same file name
		// Skip "/tmp/"
		outputFilename := filepath[5:]
		outputFilepath := coverageDataDir + "/" + string(outputFilename)
		if err := ioutil.WriteFile(outputFilepath, result, 0644); err != nil {
			testing.ContextLog(ctx, "Failed to store to "+outputFilepath)
		} else {
			hasCollected = true
		}
	}
	// If any profile is collected, also fectches the binaries of interest.
	if !hasCollected {
		return nil
	}
	for _, binaryName := range binaryNamesForCoverage {
		// Gets the binary path.
		cmd := "which " + binaryName
		binaryPath, err := d.Command("sh", "-c", cmd).CombinedOutput(ctx)
		if err != nil {
			testing.ContextLog(ctx, "Failed to run "+cmd)
			continue
		}
		cmd = "cat " + string(binaryPath)
		content, err := d.Command("sh", "-c", cmd).CombinedOutput(ctx)
		if err != nil {
			testing.ContextLog(ctx, "Failed to run "+cmd)
			continue
		}
		// Writes the content to the same file name
		outputFilepath := coveredBinaryDir + "/" + binaryName
		if err := ioutil.WriteFile(outputFilepath, content, 0755); err != nil {
			testing.ContextLog(ctx, "Failed to store to "+outputFilepath)
		}
	}

	return nil
}
