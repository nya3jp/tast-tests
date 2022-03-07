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
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
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
	bookmarkName       = "MyBookmark12345" // Arbitrary.
	shortcutName       = "MyShortcut12345" // Arbitrary.
	titleOfVersionPage = "About Version"   // chrome://version page title.
)

// prepareAshProfile resets profile migration and creates two tabs, a bookmark, and a shortcut.
// TODO(neis): Also populate Downloads.
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

	// Bookmark the chrome://version page.
	conn, err := cr.NewConn(ctx, "chrome://version")
	if err != nil {
		s.Fatal("Failed to open version page: ", err)
	}
	defer conn.Close()
	if err := kb.Accel(ctx, "Ctrl+d"); err != nil {
		s.Fatal("Failed to open bookmark creation popup: ", err)
	}
	if err := kb.Type(ctx, bookmarkName); err != nil {
		s.Fatal("Failed to type bookmark name: ", err)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)
	if err := ui.LeftClick(nodewith.Name("Done").Role(role.Button).First())(ctx); err != nil {
		s.Fatal("Failed to click button: ", err)
	}

	// Create a shortcut on the newtab page.
	if err := kb.Accel(ctx, "Ctrl+t"); err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	addShortcutButton := nodewith.Name("Add shortcut").Role(role.Button).First()
	if err := uiauto.Combine("Wait for button and click it",
		ui.WaitUntilExists(addShortcutButton),
		ui.LeftClick(addShortcutButton),
	)(ctx); err != nil {
		s.Error("Failed: ", err)
	}
	if err := kb.Type(ctx, shortcutName+"\tfoobar"); err != nil {
		s.Fatal("Failed to type shortcut data: ", err)
	}
	if err := ui.LeftClick(nodewith.Name("Done").Role(role.Button).First())(ctx); err != nil {
		s.Fatal("Failed to click button: ", err)
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

	// TODO(neis): Support -var lacrosDeployedBinary.
	if _, err := lacros.LaunchFromShelf(ctx, tconn, "/run/lacros"); err != nil {
		s.Fatal("Failed to launch lacros: ", err)
	}

	// Check that the bookmark is present.
	ui := uiauto.New(tconn)
	bookmarkedButton := nodewith.Name(bookmarkName).Role(role.Button).First()
	if err = ui.WaitUntilExists(bookmarkedButton)(ctx); err != nil {
		s.Error("Failed to find bookmark: ", err)
	}

	// Check that the shortcut is present.
	shortcutLink := nodewith.Name(shortcutName).Role(role.Link).First()
	if err := ui.WaitUntilExists(shortcutLink)(ctx); err != nil {
		s.Error("Failed to find shortcut: ", err)
	}

	// Check that there is another tab showing the version page.
	if err := kb.Accel(ctx, "Ctrl+w"); err != nil {
		s.Fatal("Failed to close tab: ", err)
	}
	if err := lacros.WaitForLacrosWindow(ctx, tconn, titleOfVersionPage); err != nil {
		s.Fatal("Failed to find appropriate window: ", err)
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
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		completedVal, err := localstate.UnmarshalPref(browser.TypeAsh, "lacros.profile_migration_completed_for_user")
		if err != nil {
			return err
		}
		completed, ok := completedVal.(map[string]bool)
		if !ok || !completed[userHash] {
			return errors.New("profile migration incomplete")
		}
		return nil
	}, nil); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Restarting after profile migration")
	return chrome.New(ctx, opts...)
}
