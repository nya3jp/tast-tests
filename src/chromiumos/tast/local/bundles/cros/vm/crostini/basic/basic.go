// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package basic

import (
	"context"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// Basic runs a simple test to ensure the command line works.
func Basic(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(vm.CrostiniPre).Container

	s.Log("Verifying pwd command works")
	cmd := cont.Command(ctx, "pwd")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run pwd: ", err)
	}
}
