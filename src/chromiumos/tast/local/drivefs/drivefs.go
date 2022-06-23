// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"os"
	"path"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
)

const (
	driveFsCommandLineArgsFileName = "command_line_args"
)

// DriveFs is a helper object for working with `drivefs` instances run within
// the Files App.
type DriveFs struct {
	user             string
	mountPath        string
	homeDir          string
	persistableToken string
}

// NewDriveFs waits for `drivefs` to mount and then creates a new `DriveFs`
// instance to work with it.
func NewDriveFs(ctx context.Context, user string) (*DriveFs, error) {
	mountPath, err := WaitForDriveFs(ctx, user)
	if err != nil {
		return nil, err
	}
	homeDir, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		return nil, err
	}
	persistableToken := PersistableToken(mountPath)
	if len(persistableToken) == 0 {
		return nil, errors.New("Failed to obtain persistable token from mount: " + mountPath)
	}
	return &DriveFs{
		user:             user,
		mountPath:        mountPath,
		homeDir:          homeDir,
		persistableToken: persistableToken,
	}, nil
}

// ConfigPath returns a path to `elem...` within the config directory for
// `drivefs`.
func (dfs *DriveFs) ConfigPath(elem ...string) string {
	return ConfigPath(dfs.homeDir, dfs.persistableToken, elem...)
}

// MyDrivePath returns a path to `elem...` within the "My Drive" aka "root"
// directory within `drivefs`.
func (dfs *DriveFs) MyDrivePath(elem ...string) string {
	return path.Join(append([]string{dfs.mountPath, "root"}, elem...)...)
}

// MountPath returns a path to `elem...` within the virtual file system provided
// by `drivefs`.
func (dfs *DriveFs) MountPath(elem ...string) string {
	return path.Join(append([]string{dfs.mountPath}, elem...)...)
}

// WriteCommandLineFlags writes a new set of command line flags for `drivefs`
// to use.
//
// Note: `Restart` should be called so `drivefs` picks up the new flags.
func (dfs *DriveFs) WriteCommandLineFlags(flags string) error {
	return os.WriteFile(dfs.ConfigPath(driveFsCommandLineArgsFileName), []byte(flags), 0644)
}

// ClearCommandLineFlags clears any previously set command line flags for
// `drivefs.
//
// Note: `Restart` should be called so `drivefs` can be restarted without flags.
func (dfs *DriveFs) ClearCommandLineFlags() error {
	return os.Remove(dfs.ConfigPath(driveFsCommandLineArgsFileName))
}

// Restart terminates the running instance of `drivefs` and waits for it to
// remount.
func (dfs *DriveFs) Restart(ctx context.Context) error {
	// Kill DriveFS, cros-disks will ensure another starts up.
	if err := testexec.CommandContext(ctx, "pkill", "-HUP", "drivefs").Run(); err != nil {
		// pkill exits with code 1 if it could find no matching process (see: man 1 pkill).
		// This is OK, as cros-disks will start one shortly.
		if ws, ok := testexec.GetWaitStatus(err); !ok || !ws.Exited() || ws.ExitStatus() != 1 {
			return errors.Wrap(err, "failed to kill drivefs processes")
		}
	}
	_, err := WaitForDriveFs(ctx, dfs.user)
	return err
}

// SaveLogsOnError saves off DriveFS logs on failure. See `SaveDriveLogsOnError`.
func (dfs *DriveFs) SaveLogsOnError(ctx context.Context, hasError func() bool) {
	if !hasError() {
		return
	}
	saveDriveLogs(ctx, dfs.homeDir, dfs.persistableToken)
}
