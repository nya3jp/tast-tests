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
var clamshellTestsForGoogleDuo = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForGoogleDuo},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForGoogleDuo = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForGoogleDuo},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleDuo,
		Desc:         "Functional test for GoogleDuo that installs the app also verifies it is logged in and that the main page is open, checks GoogleDuo correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForGoogleDuo,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForGoogleDuo,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForGoogleDuo,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForGoogleDuo,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// GoogleDuo test uses library for opting into the playstore and installing app.
// Checks GoogleDuo correctly changes the window states in both clamshell and touchview mode.
func GoogleDuo(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.apps.tachyon"
		appActivity = ".ui.main.MainActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForGoogleDuo verifies Google Duo is logged in and
// verify Google Duo reached main activity page of the app.
func launchAppForGoogleDuo(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		addPhoneNumberText      = "Add number"
		agreeButtonText         = "Agree"
		allowButtonText         = "ALLOW"
		giveAccessButtonText    = "Give access"
		searchContactsText      = "Search contacts or dial"
		videoMsgButtonClassName = "android.widget.TextView"
		videoMsgButtonText      = "Send a video message"
	)

	// Click on give access button.
	giveAccessButton := d.Object(ui.Text(giveAccessButtonText))
	if err := giveAccessButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Give Access Button doesn't exists: ", err)
	} else if err := giveAccessButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on giveAccessButton: ", err)
	}

	// Keep clicking allow button until add number button exists.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	addPhoneNumberButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(addPhoneNumberText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := addPhoneNumberButton.Exists(ctx); err != nil {
			s.Log("Click on allow button")
			allowButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
		s.Log("addPhoneNumberButton doesn't exists: ", err)
	} else {
		s.Log("addPhoneNumberButton does exists and press back")
		if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
			s.Log("Failed to enter KEYCODE_BACK: ", err)
		}
	}

	// Click on agree button.
	agreeButton := d.Object(ui.Text(agreeButtonText))
	if err := agreeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("agreeButton doesn't exists: ", err)
	} else if err := agreeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on agreeButton: ", err)
	}

	// Click on give access button.
	giveAccessButton = d.Object(ui.Text(giveAccessButtonText))
	if err := giveAccessButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Give Access Button doesn't exists: ", err)
	} else if err := giveAccessButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on giveAccessButton: ", err)
	}
	// Keep clicking allow button until add number button exists.
	allowButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	addPhoneNumberButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(addPhoneNumberText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := addPhoneNumberButton.Exists(ctx); err != nil {
			s.Log("Click on allow button")
			allowButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
		s.Log("addPhoneNumberButton doesn't exists: ", err)
	} else {
		s.Log("addPhoneNumberButton does exists and press back")
		if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
			s.Log("Failed to enter KEYCODE_BACK: ", err)
		}
	}

	// Check for add your phone number.
	addPhoneNumberButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(addPhoneNumberText))
	if err := addPhoneNumberButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("AddPhoneNumberButton doesn't exists: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
		s.Log("Failed to enter KEYCODE_BACK: ", err)
	}

	// Check for search contacts.
	checkForSearchContacts := d.Object(ui.Text(searchContactsText))
	if err := checkForSearchContacts.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("CheckForSearchContacts doesn't exists: ", err)
	}

	// Check for video message button.
	videoMessageButton := d.Object(ui.ClassName(videoMsgButtonClassName), ui.Text(videoMsgButtonText))
	if err := videoMessageButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("VideoMessageButton doesn't exists: ", err)
	}
}
