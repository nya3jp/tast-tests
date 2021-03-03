// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package remotetestutils provides utility functions for Nearby Share tests.
package remotetestutils

import (
	"context"
	"path/filepath"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// SaveLogs is a helper function to save the relevant Nearby Share logs from each DUT during a remote test.
func SaveLogs(ctx context.Context, dut *dut.DUT, tag, outDir string) error {
	var firstErr error
	logsToSave := []string{nearbyshare.ChromeLog, nearbyshare.MessageLog}
	logFiles, err := dut.Conn().Command("ls", nearbyshare.NearbyLogDir).Output(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get list of log files in remote DUTs nearby temp dir: ", err)
	} else {
		testing.ContextLog(ctx, "Files in remote DUTs nearby temp dir: ", strings.Replace(string(logFiles), "\n", " ", -1))
	}
	for _, log := range logsToSave {
		logPathSrc := filepath.Join(nearbyshare.NearbyLogDir, log)
		logPathDst := filepath.Join(outDir, log+"_"+tag)
		if err := linuxssh.GetFile(ctx, dut.Conn(), logPathSrc, logPathDst); err != nil {
			testing.ContextLogf(ctx, "Failed to save %s to %s. Error: %s", logPathSrc, logPathDst, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
