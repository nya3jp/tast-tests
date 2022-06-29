// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// A file on Google Drive might take longer to synchronize with Files app.
const driveSyncTimeout = 3 * time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchGoogleDocument,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "App Launcher Search: Google Document in Drive",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"victor.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "drivefs", "chrome_internal"},
		Fixture:      "driveFsStarted",
		Timeout:      2*time.Minute + driveSyncTimeout,
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val:  launcher.TestCase{TabletMode: false},
		}, {
			Name:              "tablet_mode",
			Val:               launcher.TestCase{TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// SearchGoogleDocument tests that App Launcher Search: Google Document in Drive.
func SearchGoogleDocument(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*drivefs.FixtureData).Chrome
	tconn := s.FixtValue().(*drivefs.FixtureData).TestAPIConn
	apiClient := s.FixtValue().(*drivefs.FixtureData).APIClient

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to take keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Ensure the name of gDoc file is unique by combine a long string, timestamp and a random number.
	gDocFilename := fmt.Sprintf("searchDrive_test_file-%020d-%06d", time.Now().UnixNano(), rand.Intn(100000))
	drivePath := filepath.Join(s.FixtValue().(*drivefs.FixtureData).MountPath, "root", gDocFilename+".gdoc")

	file, err := apiClient.CreateBlankGoogleDoc(ctx, gDocFilename, []string{"root"})
	if err != nil {
		s.Fatal("Failed to create blank google doc: ", err)
	}
	defer apiClient.RemoveFileByID(cleanupCtx, file.Id)

	s.Logf("Waiting for the file %q to exist in Files app", drivePath)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(drivePath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return err
			}
			return testing.PollBreak(err)
		}
		return nil
	}, &testing.PollOptions{Timeout: driveSyncTimeout, Interval: 3 * time.Second}); err != nil {
		s.Fatalf("Failed to wait for file %q to be available: %v", drivePath, err)
	}
	s.Logf("File %q is available in Files app", drivePath)

	tabletMode := s.Param().(launcher.TestCase).TabletMode

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure %s: %v", tabletMode, err)
	}
	defer cleanup(cleanupCtx)

	if !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed: ", err)
		}
	}

	// The expected result will not be an app, so launcher.SearchAndLaunchWithQuery and other similar functions do not work.
	if err := uiauto.Combine(fmt.Sprintf("search %q in launcher", gDocFilename),
		launcher.Open(tconn),
		launcher.Search(tconn, kb, gDocFilename),
	)(ctx); err != nil {
		s.Fatalf("Failed to search %s in launcher: %v", gDocFilename, err)
	}

	resultFinder := launcher.SearchResultListItemFinder.NameRegex(regexp.MustCompile(`^` + gDocFilename)).First()
	ui := uiauto.New(tconn)

	if err := ui.LeftClick(resultFinder)(ctx); err != nil {
		s.Fatalf("Failed to left click %s in launcher: %v", gDocFilename, err)
	}
	defer ash.CloseAllWindows(cleanupCtx, tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "launched_result_ui_dump")

	browserRootFinder := nodewith.Role(role.Window).HasClass("BrowserRootView")
	expectedNode := browserRootFinder.NameRegex(regexp.MustCompile(fmt.Sprintf("^%s - Google Docs - Google Chrome - .*", gDocFilename)))

	if err := uiauto.New(tconn).WaitUntilExists(expectedNode)(ctx); err != nil {
		s.Fatal("Failed to verify search result: ", err)
	}
}
