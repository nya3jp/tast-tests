// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IconAndUsername,
		Desc:         "Test Terminal icon on shelf and username in Terminal window",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Vars:         []string{"keepState"},
		Params:       crostini.MakeTestParams(crostini.TestInformational),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func IconAndUsername(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cr := s.PreValue().(crostini.PreData).Chrome
	keyboard := s.PreValue().(crostini.PreData).Keyboard
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData).Container)

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(ctx, tconn, strings.Split(cr.User(), "@")[0])
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}
	defer func() {
		// Exiting Terminal app.
		if err := terminalApp.Exit(cleanupCtx, keyboard); err != nil {
			s.Fatal("Failed to exit Terminal window: ", err)
		}
	}()

	// Check Terminal app is on shelf.
	if err := ash.WaitForApp(ctx, tconn, apps.Terminal.ID); err != nil {
		s.Fatal("Failed to find Terminal icon on shelf: ", err)
	}

	// TODO(jinrongwu): verify the icon of Crostini Terminal app.
}
