// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"os"
	"path"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func isRemoteTest(ctx context.Context) bool {
	out, err := testexec.CommandContext(ctx, "which", "cryptohome").Output()
	return err != nil || len(out) == 0
}

const (
	coverageDataDir  = "coverage_data/profraws"
	coveredBinaryDir = "coverage_data/binaries"
)

var (
	daemonNamesForCoverage = []string{"tpm_managerd", "attestationd", "cryptohomed"}
	binaryNamesForCoverage = []string{"tpm_managerd", "attestationd", "cryptohomed", "local_data_migration"}
	// modify this flag to true to switch on coverage profiles.
	disableCoverageCollecting = false
)

func flushCoverageData(ctx context.Context, s *testing.State) error {
	if disableCoverageCollecting {
		return errors.New("coverage collection is disabled")
	}
	if !isRemoteTest(ctx) {
		return errors.New("reboot operation only supported in remote test")
	}
	os.MkdirAll(coverageDataDir, 0777)
	os.MkdirAll(coveredBinaryDir, 0777)
	d := s.DUT()
	for _, daemon := range daemonNamesForCoverage {
		cmd := "restart " + daemon
		if _, err := d.Command("sh", "-c", cmd).CombinedOutput(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to restart "+cmd)
			continue
		}
	}
	if err := collectProfraws(ctx, s); err != nil {
		return errors.Wrap(err, "failed to collect profraw files")
	}
	return nil
}

func collectProfraws(ctx context.Context, s *testing.State) error {
	if !isRemoteTest(ctx) {
		return errors.New("reboot operation only supported in remote test")
	}
	d := s.DUT()
	// TODO(b/143522744): Restrict the set of filenames that is covered by the
	// command below. Currently there's no default llvm profile data path, so
	// it can be anywhere. The command below is designed to work with
	// situations whereby human intervention sets the llvm profile data path.
	cmd := "find /tmp -type f -path /tmp*profraw"
	result, err := d.Command("sh", "-c", cmd).CombinedOutput(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to check if any profraws in /tmp")
	}
	hasCollected := false
	for _, filePath := range strings.Fields(string(result)) {
		filePathStr := string(filePath)
		dst := path.Join(coverageDataDir, path.Base(filePathStr))
		if err = d.GetFile(ctx, filePathStr, dst); err != nil {
			// Don't stop just because a transfer failed.
			testing.ContextLog(ctx, "Failed to fetch "+filePathStr)
			continue
		}
		hasCollected = true
		// If it's successful, we can remove the profraw file from DUT.
		if result, err = d.Command("rm", "-f", filePathStr).CombinedOutput(ctx); err != nil {
			// Report the non-fatal error.
			testing.ContextLog(ctx, "Failed to remove "+filePathStr)
		}
	}
	// If any profile is collected, also fetches the binaries of interest.
	if !hasCollected {
		return nil
	}
	for _, binaryName := range binaryNamesForCoverage {
		// Gets the binary path.
		cmd := "which " + binaryName
		binaryPath, err := d.Command("sh", "-c", cmd).CombinedOutput(ctx)
		// which will annoyingly provides an extra newline at the end, so we'll
		// have to strip it.
		binaryPathStr := strings.TrimSuffix(string(binaryPath), "\n")
		if err != nil {
			testing.ContextLog(ctx, "Failed to run "+cmd)
			continue
		}
		dst := path.Join(coveredBinaryDir, path.Base(binaryPathStr))
		if err = d.GetFile(ctx, binaryPathStr, dst); err != nil {
			testing.ContextLog(ctx, "Failed to fetch "+binaryPathStr)
			continue
		}
	}

	return nil
}
