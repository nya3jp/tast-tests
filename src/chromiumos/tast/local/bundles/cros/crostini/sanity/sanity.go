// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sanity provides the most basic test for crostini containers, that a
// known command (pwd) will run.
package sanity

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// RunTest runs a simple test to ensure the command line works.
func RunTest(ctx context.Context, s *testing.State, cont *vm.Container) {
	s.Log("Verifying pwd command works")
	if err := cont.Command(ctx, "pwd").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run pwd: ", err)
	}
}
