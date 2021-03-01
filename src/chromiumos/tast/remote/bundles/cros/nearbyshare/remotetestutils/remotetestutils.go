// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package remotetestutils provides utility functions for Nearby Share tests.
package remotetestutils

import (
	"context"
	"path/filepath"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// SaveLogs is a helper function to save the relevant Nearby Share logs from each DUT during a remote test.
func SaveLogs(ctx context.Context, dut *dut.DUT, tag, outDir string) error {
	logsToSave := []string{nearbyshare.ChromeLog, nearbyshare.MessageLog}
	for _, log := range logsToSave {
		logPathSrc := filepath.Join(nearbyshare.NearbyLogDir, log)
		logPathDst := filepath.Join(outDir, log+"_"+tag)
		if err := linuxssh.GetFile(ctx, dut.Conn(), logPathSrc, logPathDst); err != nil {
			testing.ContextLogf(ctx, "Failed to save %s to %s. Error: %s", logPathSrc, logPathDst, err)
			return errors.Wrapf(err, "failed to save %s to %s", logPathSrc, logPathDst)
		}
	}
	return nil
}
