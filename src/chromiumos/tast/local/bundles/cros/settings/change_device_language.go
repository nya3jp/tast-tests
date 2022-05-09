// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChangeDeviceLanguage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Change device language and validate new langauge after reboot",
		Contacts:     []string{"shengjun@google.com", "cros-borders@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func ChangeDeviceLanguage(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	ui := uiauto.New(tconn)
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osLanguages/languages", ui.Exists(nodewith.Name("Add languages").Role(role.Button)))
	if err != nil {
		s.Fatal("Failed to open language page: ", err)
	}
	defer settings.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	if err := settings.ChangeDeviceLanguageAndReboot(ctx, tconn, "chinese", "Chinese (Simplified)"); err != nil {
		s.Fatal("Failed to change device language: ", err)
	}

	// Sleep a short time to ensure reboot button is safely clicked.
	testing.Sleep(ctx, 1*time.Second)

	cr, err = chrome.New(ctx, chrome.KeepState(), chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err = cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if err := uiauto.New(tconn).WaitUntilExists(nodewith.Name("关机"))(ctx); err != nil {
		s.Fatal("New language is not used after changing device language: ", err)
	}
}
