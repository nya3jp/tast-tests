// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package snapshot provides set of util functions for crosvm snapshot.
package snapshot

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
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
