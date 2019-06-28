// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"time"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniStartBasic,
		Desc:         "Tests basic Crostini startup only",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{vm.ContainerImageArtifact},
		Pre:          vm.CrostiniStartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func CrostiniStartBasic(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(vm.CrostiniPre).Container

	s.Log("Verifying pwd command works")
	cmd := cont.Command(ctx, "pwd")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run pwd: ", err)
	}
}
