// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/localstate"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Migrate,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test basic functionality of Ash-to-Lacros profile migration",
		Contacts: []string{
			"neis@google.com", // Test author
			"ythjkt@google.com",
			"hidehiko@google.com",
			"lacros-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Vars:         []string{"lacrosDeployedBinary"},
		Params: []testing.Param{{
			Name: "copy",
			Val:  []chrome.Option{chrome.DisableFeatures("LacrosMoveProfileMigration")},
		}, {
			Name: "move",
			Val:  []chrome.Option{chrome.EnableFeatures("LacrosMoveProfileMigration")},
		}},
	})
}

func Migrate(ctx context.Context, s *testing.State) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	prepareAshProfile(ctx, s, kb)
	cr, err := migrateProfile(ctx, s.Param().([]chrome.Option))
	if err != nil {
		s.Fatal("Failed to migrate profile: ", err)
	}
	defer cr.Close(ctx)
	verifyLacrosProfile(ctx, s, kb, cr)
}

const (
	bookmarkName         = "MyBookmark12345"                  // Arbitrary.
	extensionName        = "User-Agent Switcher for Chrome"   // Arbitrary extension from Chrome Store.
	extensionID          = "djflhoibgkdhkhhcedjiklpkjnoahfmg" // ID of the above extension.
	shortcutName         = "MyShortcut12345"                  // Arbitrary.
	titleOfDownloadsPage = "Downloads"                        // chrome://downloads page title.
)

// prepareAshProfile resets profile migration and creates two tabs, a bookmark, a download, and a shortcut.
func prepareAshProfile(ctx context.Context, s *testing.State, kb *input.KeyboardEventWriter) {
	// First restart Chrome with Lacros disabled in order to reset profile migration.
	cr, err := chrome.New(ctx, chrome.DisableFeatures("LacrosSupport"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat("/home/chronos/user/lacros/First Run"); !os.IsNotExist(err) {
			return errors.Wrap(err, "'First Run' file exists or cannot be read")
		}
		return nil
	}, nil); err != nil {
		s.Fatal("'First Run' file exists or cannot be read: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)

	// Install an extension.
	if err := policyutil.EnsureGoogleCookiesAccepted(ctx, cr.Browser()); err != nil {
		s.Fatal("Failed to accept cookies: ", err)
	}
	extensionURL := "https://chrome.google.com/webstore/detail/" + extensionID + "?hl=en"
	conn, err := cr.NewConn(ctx, extensionURL)
	if err != nil {
		s.Fatal("Failed to open extension page: ", err)
	}
	defer conn.Close()
	addButton1 := nodewith.Name("Add to Chrome").Role(role.Button).First()
	addButton2 := nodewith.Name("Add extension").Role(role.Button)
	removeButton := nodewith.Name("Remove from Chrome").Role(role.Button).First()
	if err := uiauto.Combine("Install extension",
		ui.LeftClick(addButton1),
		// The "Add extension" button may not immediately be clickable.
		ui.LeftClickUntil(addButton2, ui.Gone(addButton2)),
		ui.WaitUntilExists(removeButton),
	)(ctx); err != nil {
		s.Fatal("Failed to install: ", err)
	}

	// Bookmark the chrome://downloads page.
	if err := conn.Navigate(ctx, "chrome://downloads"); err != nil {
		s.Fatal("Failed to open downloads page: ", err)
	}
	if err := kb.Accel(ctx, "Ctrl+d"); err != nil {
		s.Fatal("Failed to open bookmark creation popup: ", err)
	}
	if err := kb.Type(ctx, bookmarkName); err != nil {
		s.Fatal("Failed to type bookmark name: ", err)
	}
	doneButton := nodewith.Name("Done").Role(role.Button)
	if err := uiauto.Combine("Click 'Done' button",
		ui.LeftClick(doneButton),
		ui.WaitUntilGone(doneButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click: ", err)
	}

	// Also download that page.
	if err := kb.Accel(ctx, "Ctrl+s"); err != nil {
		s.Fatal("Failed to open download popup: ", err)
	}
	saveButton := nodewith.Name("Save").Role(role.Button)
	if err := uiauto.Combine("Click 'Save' button",
		ui.LeftClick(saveButton),
		ui.WaitUntilGone(saveButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click: ", err)
	}

	// Create a shortcut on the newtab page.
	if err := kb.Accel(ctx, "Ctrl+t"); err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	addShortcutButton := nodewith.Name("Add shortcut").Role(role.Button)
	if err := uiauto.Combine("Click 'Add shortcut' button",
		ui.LeftClick(addShortcutButton),
		ui.WaitUntilGone(addShortcutButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click: ", err)
	}
	if err := kb.Type(ctx, shortcutName+"\tfoobar"); err != nil {
		s.Fatal("Failed to type shortcut data: ", err)
	}
	if err := uiauto.Combine("Click 'Done' button",
		ui.LeftClick(doneButton),
		ui.WaitUntilGone(doneButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click: ", err)
	}
}

// verifyLacrosProfile checks that the edits done by prepareAshProfile were carried over to Lacros.
func verifyLacrosProfile(ctx context.Context, s *testing.State, kb *input.KeyboardEventWriter, cr *chrome.Chrome) {
	if _, err := os.Stat("/home/chronos/user/lacros/First Run"); err != nil {
		s.Fatal("Error reading 'First Run' file: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	l, err := lacros.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch lacros: ", err)
	}

	// Check that the bookmark is present.
	ui := uiauto.New(tconn)
	bookmarkedButton := nodewith.Name(bookmarkName).Role(role.Button)
	if err = ui.WaitUntilExists(bookmarkedButton)(ctx); err != nil {
		s.Error("Failed to find bookmark: ", err)
	}

	// Check that the shortcut is present.
	shortcutLink := nodewith.Name(shortcutName).Role(role.Link)
	if err := ui.WaitUntilExists(shortcutLink)(ctx); err != nil {
		s.Error("Failed to find shortcut: ", err)
	}

	// Check that there is another tab showing the downloads page.
	if err := kb.Accel(ctx, "Ctrl+w"); err != nil {
		s.Fatal("Failed to close tab: ", err)
	}
	if err := lacros.WaitForLacrosWindow(ctx, tconn, titleOfDownloadsPage); err != nil {
		s.Fatal("Failed to find appropriate window: ", err)
	}

	// Check that the download page shows the previous download (of itself).
	downloadedFile := nodewith.Name(titleOfDownloadsPage + ".mhtml").Role(role.Link)
	if err = ui.WaitUntilExists(downloadedFile)(ctx); err != nil {
		s.Error("Failed to find download: ", err)
	}

	// Check that the extension is installed and enabled.
	conn, err := l.NewConn(ctx, "chrome://extensions/?id="+extensionID)
	if err != nil {
		s.Fatal("Failed to open extension page: ", err)
	}
	defer conn.Close()
	extensionText := nodewith.Name(extensionName).Role(role.StaticText)
	onText := nodewith.Name("On").Role(role.StaticText)
	if err := uiauto.Combine("Verify extension status",
		ui.WaitUntilExists(extensionText),
		ui.Exists(onText),
	)(ctx); err != nil {
		s.Error("Failed: ", err)
	}
}

func migrateProfile(ctx context.Context, extraOpts []chrome.Option) (*chrome.Chrome, error) {
	// TODO(chromium:1290297): This is a hack.
	// chrome.New doesn't really support profile migration because it
	// doesn't anticipate the additional Chrome restart that profile
	// migration effects. As a result, the *Chrome return value is already
	// invalid and we must not use it. Moreover, we must disable the
	// RemoveNotification option because otherwise chrome.New will try to
	// interact with Chrome at a time when that is no longer safe.
	// In order to obtain a valid *Chrome value for the test to continue
	// with, we restart Chrome once more after profile migration.
	testing.ContextLog(ctx, "Restarting for profile migration")
	opts := []chrome.Option{
		chrome.KeepState(),
		chrome.EnableFeatures("LacrosSupport", "LacrosPrimary", "LacrosProfileMigrationForAnyUser"),
		chrome.ExtraArgs("--lacros-selection=rootfs"),
		chrome.LacrosExtraArgs("--remote-debugging-port=0"),
		chrome.RemoveNotification(false),
	}
	opts = append(opts, extraOpts...)
	crDoNotUse, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, err
	}
	defer crDoNotUse.Close(ctx)

	testing.ContextLog(ctx, "Waiting for profile migration to complete")
	userHash, err := cryptohome.UserHash(ctx, chrome.DefaultUser)
	if err != nil {
		return nil, err
	}
	pref := "lacros.profile_migration_completed_for_user." + userHash
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		completedVal, err := localstate.UnmarshalPref(browser.TypeAsh, pref)
		if err != nil {
			return err
		}
		completed, ok := completedVal.(bool)
		if !ok || !completed {
			return errors.New("profile migration incomplete")
		}
		return nil
	}, nil); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Restarting after profile migration")
	return chrome.New(ctx, opts...)
}
