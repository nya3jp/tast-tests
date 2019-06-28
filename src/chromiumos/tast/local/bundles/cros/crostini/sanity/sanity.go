// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sanity

import (
	"context"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Sanity runs a simple test to ensure the command line works.
func Sanity(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container

	s.Log("Verifying pwd command works")
	if err := cont.Command(ctx, "pwd").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run pwd: ", err)
	}
}
