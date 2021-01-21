// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dlc provides ways to interact with dlcservice daemon and utilities.
package dlc

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Dlcservice related constants.
const (
	CacheDir    = "/var/cache/dlc"
	JobName     = "dlcservice"
	LibDir      = "/var/lib/dlcservice/dlc"
	PreloadDir  = "/var/cache/dlc-images"
	ServiceName = "org.chromium.DlcService"
	User        = "dlcservice"
)

// Info holds the fields related to a DLC.
type Info struct {
	ID      string
	Package string
}

// Install calls the DBus method to install a DLC.
func Install(ctx context.Context, id, omahaURL string) error {
	testing.ContextLog(ctx, "Installing DLC: ", id, " using ", omahaURL)
	if err := testexec.CommandContext(ctx, "dlcservice_util", "--install", "--id="+id, "--omaha_url="+omahaURL).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to install")
	}
	return nil
}

// Purge calls the DBus method to Purge a DLC.
func Purge(ctx context.Context, id string) error {
	testing.ContextLog(ctx, "Purging DLC: ", id)
	if err := testexec.CommandContext(ctx, "dlcservice_util", "--purge", "--id="+id).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to purge")
	}
	return nil
}

// Uninstall calls the DBus method to uninstall a DLC
func Uninstall(ctx context.Context, id string) error {
	testing.ContextLog(ctx, "Uninstalling DLC: ", id)
	if err := testexec.CommandContext(ctx, "dlcservice_util", "--uninstall", "--id="+id).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to uninstall")
	}
	return nil
}

// Cleanup removes all DLC related states and restarts dlcservice. Note that this will delete the DLC image from disk!
func Cleanup(ctx context.Context, infos ...Info) error {
	for _, info := range infos {
		// Unmount the DLC.
		path := filepath.Join("/run/imageloader", info.ID, info.Package)
		if err := testexec.CommandContext(ctx, "imageloader", "--unmount", "--mount_point="+path).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to unmount DLC (%s)", info.ID)
		}
		// Remove all related directories.
		for _, dir := range []string{CacheDir, LibDir, PreloadDir} {
			if err := os.RemoveAll(filepath.Join(dir, info.ID)); err != nil {
				return errors.Wrapf(err, "failed to cleanup directory (%s)", dir)
			}
		}
	}
	if err := upstart.RestartJobAndWaitForDbusService(ctx, JobName, ServiceName); err != nil {
		return errors.Wrap(err, "failed to restart dlcservice")
	}
	return nil
}
