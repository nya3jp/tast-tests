// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultAppLaunchWhenArcIsOff,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify Default App Icons Launch Opt In Flow When PlayStore is Off ",
		Contacts:     []string{"cpiao@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func DefaultAppLaunchWhenArcIsOff(ctx context.Context, s *testing.State) {
	const (
		defaultTimeout = 20 * time.Second
	)

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Launch Play Books App.
	if err := launcher.SearchAndWaitForAppOpen(tconn, kb, apps.PlayBooks)(ctx); err != nil {
		s.Log("Failed to Launch the Play Books: ", err)
	}

	ui := uiauto.New(tconn)
	more := nodewith.Name("More").Role(role.StaticText)
	accept := nodewith.Name("Accept").Role(role.Button)

	if err := uiauto.Combine("verify optin flow launch",
		ui.WaitUntilExists(more),
		ui.LeftClick(more),
		ui.WaitUntilExists(accept),
		ui.LeftClick(accept),
	)(ctx); err != nil {
		s.Fatal("Failed to launch optin flow: ", err)
	}

	// Verify Play Store is Enabled.
	if err = optin.WaitForPlayStoreReady(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store to be ready: ", err)
	}

}
