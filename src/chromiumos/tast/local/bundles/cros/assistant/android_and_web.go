// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
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
		ApkName                = "AssistantAndroidAppTest.apk"
	)

	predata := s.PreValue().(arc.PreData)
	cr := predata.Chrome
	a := predata.ARC

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

	if err := a.Install(ctx, arc.APKPath(ApkName)); err != nil {
		s.Fatal("Failed to install a test app: ", err)
	}

	if err := pollForArcPackageAvailable(ctx, s, tconn, YtPackageName); err != nil {
		s.Fatal("Failed to wait arc package becomes available: ", err)
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

type arcPackageDict struct {
	PackageName         string  `json:"packageName"`
	PackageVersion      int64   `json:"packageVersion"`
	LastBackupAndroidID string  `json:"lastBackupAndroidId"`
	LastBackupTime      float64 `json:"lastBackupTime"`
	ShouldSync          bool    `json:"shouldSync"`
	System              bool    `json:"system"`
	VpnProvider         bool    `json:"vpnProvider"`
}

func pollForArcPackageAvailable(ctx context.Context, s *testing.State, tconn *chrome.TestConn, packageName string) error {
	f := func(ctx context.Context) error {
		var packageDict arcPackageDict
		return tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getArcPackage.bind(this,"`+packageName+`"))()`, &packageDict)
	}
	return testing.Poll(ctx, f, &testing.PollOptions{})
}
