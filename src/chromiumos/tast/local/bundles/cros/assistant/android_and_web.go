// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AndroidAndWeb,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test assistant to open Android app over web app",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"yawano@google.com", "assistive-eng@google.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		VarDeps:      []string{"assistant.username", "assistant.password"},
		Pre: arc.NewPrecondition("assistant",
			&arc.GaiaVars{
				UserVar: "assistant.username",
				PassVar: "assistant.password",
			},
			nil,   // Gaia login pool
			false, // Whether crosvm to use O_DIRECT
			"--arc-disable-app-sync",
		),
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 3*time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func AndroidAndWeb(ctx context.Context, s *testing.State) {
	const (
		QueryOpenYt            = "Open YouTube"
		WebYtTitle             = "Chrome - YouTube"
		YtPackageName          = "com.google.android.youtube"
		PlayStoreLaunchTimeout = time.Minute
	)

	predata := s.PreValue().(arc.PreData)
	cr := predata.Chrome
	a := predata.ARC

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to create UIDevice: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := assistant.EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	defer func() {
		if err := assistant.Cleanup(ctx, s.HasError, cr, tconn); err != nil {
			s.Fatal("Failed to disable Assistant: ", err)
		}
	}()

	if _, err := assistant.SendTextQuery(ctx, tconn, QueryOpenYt); err != nil {
		s.Fatal("Failed to send Assistant text query: ", err)
	}

	predYtWeb := func(window *ash.Window) bool {
		return window.Title == WebYtTitle && window.IsVisible && window.ARCPackageName == ""
	}
	if err := ash.WaitForCondition(ctx, tconn, predYtWeb, &testing.PollOptions{}); err != nil {
		s.Fatal("Failed to confirm that YouTube web page gets opened: ", err)
	}

	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to close all windows: ", err)
	}

	if err := optin.LaunchAndWaitForPlayStore(ctx, tconn, cr, PlayStoreLaunchTimeout); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}

	if err := playstore.InstallApp(ctx, a, d, YtPackageName, &playstore.Options{}); err != nil {
		s.Fatal("Failed to install YouTube app: ", err)
	}

	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to close all windows: ", err)
	}

	if _, err := assistant.SendTextQuery(ctx, tconn, QueryOpenYt); err != nil {
		s.Fatal("Failed to send Assistant text query: ", err)
	}

	predYtApp := func(window *ash.Window) bool {
		return window.IsVisible && window.ARCPackageName == YtPackageName
	}
	if err := ash.WaitForCondition(ctx, tconn, predYtApp, &testing.PollOptions{}); err != nil {
		s.Fatal("Failed to confirm that YouTube app gets opened: ", err)
	}
}
