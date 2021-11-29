// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchHelpAppOffline,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Help app can be launched offline with bundled content",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org", // Test author
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(pre.AppsStableModels),
	})
}

func LaunchHelpAppOffline(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer shortCancel()

	// Must use new chrome instance to make sure help app never launched.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	categoryCardFinder := nodewith.Role(role.Paragraph).ClassName("category")

	ui := uiauto.New(tconn)
	helpCtx := helpapp.NewContext(cr, tconn)

	offlineSteps := uiauto.Combine("test offline action",
		helpCtx.Launch(),
		// Wait for card displayed.
		ui.WaitUntilExists(categoryCardFinder.First()),
		// All showoff-card category names can only be "HELP".
		func(ctx context.Context) error {
			expr := `
				var nodes = shadowPiercingQueryAll(".category");
				var unexpectedCategories = [];
				nodes.forEach(node=>{
					if(node.innerText != "HELP" && !unexpectedCategories.includes(node.innerText)){
						unexpectedCategories.push(node.innerText);
					}
				});
				if (unexpectedCategories.length>0){
					throw new Error("Cards should not be shown offline: " + unexpectedCategories.join(","))
				}`
			return helpCtx.EvalJSWithShadowPiercer(ctx, expr, nil)
		},
	)

	// Run test steps in offline mode.
	if err := network.ExecFuncOnChromeOffline(ctx, offlineSteps); err != nil {
		s.Error("Failed to verify Help app running offline: ", err)
	}
}
