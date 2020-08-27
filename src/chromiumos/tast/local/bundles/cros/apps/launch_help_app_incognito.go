// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpAppIncognito,
		Desc: "Help app can be launched in incognito mode",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: pre.AppsStableModels,
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
			},
		},
	})
}

// LaunchHelpAppIncognito verifies launching Help app in incognito mode.
func LaunchHelpAppIncognito(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Press Ctrl+Shift+N to launch a new browser in incognito mode
	if err := kb.Accel(ctx, "Ctrl+Shift+N"); err != nil {
		s.Fatal("Failed to send Ctrl+Shift+N: ", err)
	}

	incognitoParams := ui.FindParams{
		Name:      "Incognito",
		ClassName: "AvatarToolbarButton",
	}
	if _, err := ui.FindWithTimeout(ctx, tconn, incognitoParams, 30*time.Second); err != nil {
		s.Fatal("Failed to launch browser in incognito mode: ", err)
	}

	if err := helpapp.LaunchFromThreeDotMenu(ctx, tconn); err != nil {
		s.Fatal("Failed to launch Help app from chrome three dot menu: ", err)
	}
}
