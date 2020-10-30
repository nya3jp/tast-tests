// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dev

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	// Example usage:
	// $ tast run -var=user=<username> -var=pass=<password> <dut ip> dev.SnapshotData
	// <username> and <password> are the credentials of the test GAIA
	// account. After the test the contents of android-data/data will be
	// left as snapshot_data.tar.gz in the MyFiles directory for
	// <username>.
	testing.AddTest(&testing.Test{
		Func:         SnapshotData,
		Desc:         "Connect to Chrome Remote Desktop for working remotely",
		Contacts:     []string{"ereth@google.com", "arc-core@google.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		Vars:         []string{"user", "pass"},
		Timeout:      5 * time.Minute,
	})
}

func SnapshotData(ctx context.Context, s *testing.State) {
	user := s.RequiredVar("user")
	pass := s.RequiredVar("pass")

	cr, err := chrome.New(ctx, chrome.KeepState(), chrome.GAIALogin(), chrome.Auth(user, pass, ""), chrome.ARCDisabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Tar up all the Android data.
	if err := snapshotData(ctx, s, cr.User()); err != nil {
		s.Fatal("Snapshotting failed: ", err)
	}
}

func snapshotData(ctx context.Context, s *testing.State, user string) error {
	s.Log("Creating data snapshot")

	systemPath, err := cryptohome.SystemPath(user)
	if err != nil {
		return errors.Wrap(err, "cannot get system path for user")
	}
	androidPath := filepath.Join(systemPath, "android-data")

	userPath, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		return errors.Wrap(err, "cannot get user path for user")
	}
	snapshotPath := filepath.Join(userPath, "MyFiles/snapshot_data.tar.gz")

	cmd := testexec.CommandContext(ctx, "tar", "--selinux", "--xattrs", "--numeric-owner", "-czf", snapshotPath, "-C", androidPath, "data")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to create android data snapshot")
	}

	s.Log("Snapshot created at " + snapshotPath)

	return nil
}
