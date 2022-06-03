// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

// PersistableToken derives the token from the mount path. This is used
// to identify the user account directory under ~/GCache/v2.
func PersistableToken(mountPath string) string {
	return strings.TrimPrefix(mountPath, "/media/fuse/drivefs-")
}

// ConfigPath returns the path to `elem...` in the DriveFS
// configuration directory based on the provided `homeDir` and
// `persistableToken`.
func ConfigPath(homeDir, persistableToken string, elem ...string) string {
	return path.Join(append([]string{homeDir, "GCache", "v2", persistableToken}, elem...)...)
}

// SaveDriveLogsOnError dumps the drivefs logs to the context output dir to
// enable ease of debugging when an error occurs. This should be deferred on any
// test that you wish to obtain logs on error.
func SaveDriveLogsOnError(ctx context.Context, hasError func() bool, normalizedUser, mountPath string) {
	if !hasError() {
		return
	}
	persistableToken := PersistableToken(mountPath)
	if len(persistableToken) == 0 {
		testing.ContextLog(ctx, "Could not obtain the drive persistable token from mount path: ", mountPath)
		return
	}
	homeDir, err := cryptohome.UserPath(ctx, normalizedUser)
	if err != nil {
		testing.ContextLog(ctx, "Could not obtain the home dir path: ", err)
		return
	}
	driveLogPath := ConfigPath(homeDir, persistableToken, "Logs", "drivefs.txt")
	logContents, err := ioutil.ReadFile(driveLogPath)
	if err != nil {
		testing.ContextLogf(ctx, "Could not read the Drive log %q: %v", driveLogPath, err)
		return
	}
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		testing.ContextLog(ctx, "Could not obtain the context out dir")
		return
	}
	if err = ioutil.WriteFile(filepath.Join(outDir, "drivefs_logs.txt"), logContents, 0644); err != nil {
		testing.ContextLog(ctx, "Could not write the Drive log to out dir: ", err)
	}
}
