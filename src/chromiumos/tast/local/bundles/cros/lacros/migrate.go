// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
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
			Val:  []chrome.Option{},
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
	verifyLacrosProfile(ctx, s, kb, cr)
}

// prepareAshProfile resets profile migration and creates two tabs, a bookmark, and a shortcut.
func prepareAshProfile(ctx context.Context, s *testing.State, kb *input.KeyboardEventWriter) {
	// First restart Chrome with Lacros disabled in order to reset profile migration.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	if _, err := os.Stat("/home/chronos/user/lacros/First Run"); !os.IsNotExist(err) {
		s.Fatal("'First Run' file exists or cannot be read: ", err)
	}

	// Bookmark the chrome://version page.
	conn, err := cr.NewConn(ctx, "chrome://version")
	if err != nil {
		s.Fatal("Failed to open version page: ", err)
	}
	defer conn.Close()
	if err := kb.Accel(ctx, "Ctrl+d"); err != nil {
		s.Fatal("Failed to open bookmark added popup: ", err)
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
	if err := kb.Type(ctx, "gaga\tgugu"); err != nil {
		s.Fatal("Failed to type bookmark name: ", err)
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

	// Check that the two tabs, the bookmark, and the shortcut are present.
	ui := uiauto.New(tconn)
	bookmarkedButton := nodewith.Name(bookmarkName).Role(role.Button).First()
	if err = ui.WaitUntilExists(bookmarkedButton)(ctx); err != nil {
		s.Error("Bookmark bar with bookmarked URL not found: ", err)
	}
	addShortcutButton := nodewith.Name("gaga").Role(role.Link).First()
	if err := ui.WaitUntilExists(addShortcutButton)(ctx); err != nil {
		s.Error("Add shortcut button not found: ", err)
	}
	if err := kb.Accel(ctx, "Ctrl+w"); err != nil {
		s.Fatal("Failed to open bookmark added popup: ", err)
	}
	if err := lacros.WaitForLacrosWindow(ctx, tconn, titleOfVersionPage); err != nil {
		s.Fatal("Failed to find appropriate Lacros window: ", err)
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
	if _, err := chrome.New(ctx, opts...); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Waiting for profile migration to complete")
	userHash, err := cryptohome.UserHash(ctx, chrome.DefaultUser)
	if err != nil {
		return nil, err
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// TODO(neis): Move this Local State query functionality to a
		// common place, there are other tests doing something similar.
		command := "jq"
		args := []string{
			".lacros.profile_migration_completed_for_user[\"" + userHash + "\"]",
			"/home/chronos/Local State",
		}
		output, err := testexec.CommandContext(ctx, command, args...).Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		} else if strings.TrimSpace(string(output)) != "true" {
			return errors.Errorf("profile migration not completed, got %s", string(output))
		}
		return nil
	}, nil); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Restarting after profile migration")
	return chrome.New(ctx, opts...)
}

const titleOfVersionPage = "About Version" // Must match actual page title.
const bookmarkName = "MyBookmark12345"     // Arbitrary.
