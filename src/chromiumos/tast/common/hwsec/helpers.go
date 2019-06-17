// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

import (
	"context"
)

// DUTCommandRunner declares interface that runs command on DUT
type DUTCommandRunner interface {
	Run(ctx context.Context, cmd string, args ...string) ([]byte, error)
}

// DUTRebooter declares interface that reboots DUT
type DUTRebooter interface {
	Reboot(ctx context.Context) error
}

// Helper provides various helper functions that could be shared across all
// hwsec integration test regardless of run-type, i.e., remote or local.
type Helper struct {
	DUTCommandRunner
	DUTRebooter
}

// RunShell execute |cmd| in a new shell.
func (h *Helper) RunShell(ctx context.Context, cmd string) ([]byte, error) {
	return DUTCommandRunner(h).Run(ctx, "sh", "-c", cmd)
}
