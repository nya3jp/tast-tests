// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package guestos provides VM guest OS related primitives.
package guestos

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/vm"
)

// CrostiniGuestOS is an implementation of IGuestOS interface for Crostini
type CrostiniGuestOS struct {
	VMInstance *vm.Container
}

// Command returns a testexec.Cmd with a vsh command that will run in the guest.
func (c CrostiniGuestOS) Command(ctx context.Context, vshArgs ...string) *testexec.Cmd {
	return c.VMInstance.Command(ctx, vshArgs...)
}

// GetBinPath returns the recommended binaries path in the guest OS.
// The trace_replay binary will be uploaded into this directory.
func (c CrostiniGuestOS) GetBinPath() string {
	return "/tmp/trace_replay/bin"
}

// GetTempPath returns the recommended temp path in the guest OS. This directory
// can be used to store downloaded artifacts and other temp files.
func (c CrostiniGuestOS) GetTempPath() string {
	return "/tmp/trace_replay"
}
