// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

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
		Timeout:      5 * time.Minute,
	})
}

// SearchGoogleDocument tests that App Launcher Search: Google Document in Drive.
func SearchGoogleDocument(ctx context.Context, s *testing.State) {
	fixtureData := s.FixtValue().(*drivefs.FixtureData)
	tconn := fixtureData.TestAPIConn

	// Ensure the name of gDoc file is unique by combine a long string, timestamp and a random number.
	gDocFilename := fmt.Sprintf("searchDrive_test_file-%020d-%06d", time.Now().UnixNano(), rand.Intn(100000))
	gDocNode := nodewith.NameRegex(regexp.MustCompile(`^` + gDocFilename))

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to take keyboard: ", err)
	}
	defer kb.Close()

	apiClient := s.FixtValue().(*drivefs.FixtureData).APIClient

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	file, err := apiClient.CreateBlankGoogleDoc(ctx, gDocFilename, []string{"root"})
	if err != nil {
		s.Fatal("Failed to create blank google doc: ", err)
	}
	defer apiClient.RemoveFileByID(cleanupCtx, file.Id)

	if err := waitForGoogleDocExists(ctx, tconn, gDocFilename); err != nil {
		s.Fatalf("Failed to verify %s in drive: %q", gDocFilename, err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, fixtureData.Chrome, "search_google_document")

	searchResult := gDocNode.Role(role.ListBoxOption).HasClass("ui/app_list/SearchResultView").First()

	if err := launcher.SearchAndLeftClick(ctx, tconn, kb, gDocFilename, searchResult); err != nil {
		s.Fatal("Failed to search and left click in launcher: ", err)
	}
	defer ash.CloseAllWindows(cleanupCtx, tconn)

	if err := uiauto.New(tconn).WaitUntilExists(gDocNode.HasClass("Label"))(ctx); err != nil {
		s.Fatalf("Failed to wait %s open: %v", gDocFilename, err)
	}
}

// waitForGoogleDocExists wait the specified file exists in files app.
func waitForGoogleDocExists(ctx context.Context, tconn *chrome.TestConn, fileName string) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	file, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open filesapp")
	}
	defer file.Close(cleanupCtx)

	return uiauto.Combine("open drive and wait for "+fileName,
		file.OpenDrive(),
		file.WithTimeout(3*time.Minute).WaitForFile(fileName+".gdoc"),
	)(ctx)
}
