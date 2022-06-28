// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcDownload,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies ARC++ download behavior in holding space",
		Contacts: []string{
			"dmblack@google.com",
			"tote-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 10*time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

// ArcDownload TODO(dmblack): Document.
func ArcDownload(ctx context.Context, s *testing.State) {
	// TODO(dmblack): Document.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// TODO(dmblack): Document.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// TODO(dmblack): Document.
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	// TODO(dmblack): Document.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	// TODO(dmblack): Document.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	// TODO(dmblack): Document.
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// TODO(dmblack): Document.
	pkgName := "com.chrome.beta"
	if err := playstore.InstallApp(ctx, a, d, pkgName, &playstore.Options{}); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	// TODO(dmblack): Document.
	if err := a.Command(ctx, "am", []string{"start",
		"-p", pkgName,
		"-a", "android.intent.action.VIEW",
		"-d", "https://www.engadget.com/"}...,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to send intent to open %q: ", pkgName, err)
	}
}
