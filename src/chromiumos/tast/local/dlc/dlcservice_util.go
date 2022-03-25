// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dlc provides ways to interact with dlcservice daemon and utilities.
package dlc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Dlcservice related constants.
const (
	CacheDir          = "/var/cache/dlc"
	JobName           = "dlcservice"
	LibDir            = "/var/lib/dlcservice/dlc"
	PreloadDir        = "/var/cache/dlc-images"
	FactoryInstallDir = "/mnt/stateful_partition/unencrypted/dlc-factory-images"
	ServiceName       = "org.chromium.DlcService"
	User              = "dlcservice"
)

// Info holds the fields related to a DLC.
type Info struct {
	ID        string `json:"id"`
	Package   string `json:"package"`
	RootMount string `json:"root_mount"`
}

// State holds the fields related to the DLC state.
type State struct {
	ID            string  `json:"id"`
	LastErrorCode string  `json:"last_error_code"`
	Progress      float32 `json:"progress"`
	RootPath      string  `json:"root_path"`
	State         int     `json:"state"`
}

// Install calls the DBus method to install a DLC.
func Install(ctx context.Context, id, omahaURL string) error {
	testing.ContextLog(ctx, "Installing DLC: ", id, " using ", omahaURL)
	// TODO(b/172220710): Remove retries once util + dlcservice timeouts are fixed.
	var err error
	for i := 0; i < 3; i++ {
		err = testexec.CommandContext(ctx, "dlcservice_util", "--install", "--id="+id, "--omaha_url="+omahaURL).Run(testexec.DumpLogOnError)
		if err == nil {
			return nil
		}
	}
	return errors.Wrap(err, "failed to install")
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

// List returns all the installed DLC(s).
func List(ctx context.Context) (map[string][]Info, error) {
	buf, err := testexec.CommandContext(ctx, "dlcservice_util", "--list").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list installed DLCs")
	}

	info := make(map[string][]Info)
	if err := json.Unmarshal(buf, &info); err != nil {
		return nil, err
	}

	return info, nil
}

// GetDlcState returns the state of a DLC.
func GetDlcState(ctx context.Context, id string) (*State, error) {
	buf, err := testexec.CommandContext(ctx, "dlcservice_util", "--dlc_state", "--id="+id).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the DLC state")
	}

	var state State
	if err := json.Unmarshal(buf, &state); err != nil {
		return nil, err
	}

	return &state, nil
}
