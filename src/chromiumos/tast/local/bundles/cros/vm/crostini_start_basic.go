// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniStartBasic,
		Desc:         "Tests basic Crostini startup only",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      5 * time.Minute,
		Data:         []string{"guest_images.tar"},
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func CrostiniStartBasic(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component")
	artifactPath := s.DataPath("guest_images.tar")
	if err := vm.MountArtifactComponent(ctx, artifactPath); err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Creating default container")
	cont, err := vm.CreateArtifactContainer(ctx, s.OutDir(), cr.User(), artifactPath)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer vm.StopConcierge(ctx)
	defer func() {
		if err := cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failure dumping container log: ", err)
		}
	}()

	s.Log("Verifying pwd command works")
	cmd := cont.Command(ctx, "pwd")
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run pwd: ", err)
	}
}
