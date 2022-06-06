// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package snapshot provides set of util functions for crosvm snapshot.
package snapshot

import (
	"context"
	"path/filepath"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
)

// Status represents the current status of crosvm snapshot.
type Status int

// Status of crosvm snapshot.
const (
	NotAvailable Status = iota
	InProgress
	Done
	Failed
)

const (
	daemonStoreBase = "/run/daemon-store/crosvm"
)

// GetStatus fetches the current snapshot status from the running crosvm.
func GetStatus(ctx context.Context, socketPath string) (Status, error) {
	dump, err := testexec.CommandContext(ctx, "crosvm", "snapshot", "status", socketPath).Output()
	if err != nil {
		return NotAvailable, errors.Wrap(err, "snapshot status")
	}
	status := string(dump)
	if strings.Contains(status, "NotAvailable") {
		return NotAvailable, nil
	} else if strings.Contains(status, "InProgress") {
		return InProgress, nil
	} else if strings.Contains(status, "Done") {
		return Done, nil
	} else if strings.Contains(status, "Failed") {
		return Failed, nil
	} else {
		return NotAvailable, errors.Errorf("unexpected snapshot status: %s", status)
	}
}

// GetPath returns a path to store the snapshot.
func GetPath(ctx context.Context, user string) (string, error) {
	userHash, err := cryptohome.UserHash(ctx, user)
	if err != nil {
		return "", errors.Wrap(err, "get user hash")
	}
	return filepath.Join(daemonStoreBase, userHash), nil
}

// Take takes a snapshot of crosvm.
func Take(ctx context.Context, socketPath, snapshotPath string) error {
	if err := testexec.CommandContext(ctx, "crosvm", "snapshot", "take", socketPath, snapshotPath).Run(); err != nil {
		return errors.Wrap(err, "take snapshot")
	}
	return nil
}

// Resume resumes the suspended crosvm.
func Resume(ctx context.Context, socketPath string) error {
	if err := testexec.CommandContext(ctx, "crosvm", "resume", socketPath).Run(); err != nil {
		return errors.Wrap(err, "resume from snapshot")
	}
	return nil
}
