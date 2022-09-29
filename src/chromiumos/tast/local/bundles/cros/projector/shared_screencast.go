// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/projector"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SharedScreencast,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Opens a shared screencast in viewer mode",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-projector+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		VarDeps: []string{
			"projector.sharedScreencastLink",
		},
		Params: []testing.Param{
			{
				Fixture: "projectorLogin",
				Val:     browser.TypeAsh,
			},
			{
				Name:              "lacros",
				Fixture:           "lacrosProjectorLogin",
				ExtraSoftwareDeps: []string{"lacros"},
				Val:               browser.TypeLacros,
			},
		},
	})
}

func SharedScreencast(ctx context.Context, s *testing.State) {
	// Leave 10 seconds for dumping the UI tree.
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	sharedScreencast := s.RequiredVar("projector.sharedScreencastLink")

	defer faillog.DumpUITreeOnError(ctxForCleanUp, s.OutDir(), s.HasError, tconn)

	if err := projector.OpenSharedScreencast(ctx, tconn, cr, s.Param().(browser.Type), sharedScreencast); err != nil {
		s.Fatal("Failed to open shared screencast: ", err)
	}

	// Set timeout to one minute to allow the shared screencast to load over the network.
	ui := uiauto.New(tconn).WithTimeout(time.Minute)

	// Verify the shared screencast title rendered correctly.
	if err := ui.WaitUntilExists(nodewith.Name("Screencast for Tast (Do not modify)").Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Failed to render shared screencast: ", err)
	}
}
