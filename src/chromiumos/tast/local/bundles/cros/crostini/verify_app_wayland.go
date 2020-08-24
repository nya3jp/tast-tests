// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"

	"chromiumos/tast/local/bundles/cros/crostini/verifyapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyAppWayland,
		Desc:         "Runs a Wayland crostini application from the terminal and verifies that it renders",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Vars:         []string{"keepState"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params:       crostini.MakeTestParams(crostini.TestCritical),
	})
}

func VerifyAppWayland(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	defer crostini.RunCrostiniPostTest(ctx, pre.Container)

	verifyapp.RunTest(ctx, s, pre.Chrome, pre.Container, crostini.WaylandDemoConfig())
}
