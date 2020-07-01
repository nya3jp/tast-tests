// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForRoblox = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForRoblox},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForRoblox = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForRoblox},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Roblox,
		Desc:         "Functional test for Roblox that installs the app also verifies it is logged in and that the main page is open, checks Roblox correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForRoblox,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForRoblox,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForRoblox,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForRoblox,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Roblox.username", "arcappcompat.Roblox.password"},
	})
}

// Roblox test uses library for opting into the playstore and installing app.
// Checks Roblox correctly changes the window states in both clamshell and touchview mode.
func Roblox(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.roblox.client"
		appActivity = ".ActivityNativeMain"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForRoblox verifies Roblox is logged in and
// verify Roblox reached main activity page of the app.
func launchAppForRoblox(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		enterUserNameID   = "com.roblox.client:id/view_login_username_field"
		enterPasswordText = "Password"
		loginButtonText   = "Log In"
		loginText         = "Login"
		passwordID        = "com.roblox.client:id/view_login_password_field"
	)

	// Click on login button.
	loginButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(loginText))
	if err := loginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Login Button doesn't exists: ", err)
	} else if err := loginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on loginButton: ", err)
	}

	// Enter username.
	robloxUserName := s.RequiredVar("arcappcompat.Roblox.username")
	enterUserName := d.Object(ui.ID(enterUserNameID))
	if err := enterUserName.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("enterUserName doesn't exist: ", err)
	} else if err := enterUserName.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterUserName: ", err)
	} else if err := enterUserName.SetText(ctx, robloxUserName); err != nil {
		s.Fatal("Failed to enterUserName: ", err)
	}

	// Enter Password.
	robloxPassword := s.RequiredVar("arcappcompat.Roblox.password")
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if err := enterPassword.SetText(ctx, robloxPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on login button.
	loginButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(loginButtonText))
	if err := loginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Login Button doesn't exists: ", err)
	} else if err := loginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on loginButton: ", err)
	}
}
