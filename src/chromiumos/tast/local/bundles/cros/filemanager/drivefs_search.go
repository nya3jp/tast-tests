// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/local/bundles/cros/filemanager/pre"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DrivefsSearch,
		Desc: "Verify that exact file search for Google Drive returns correct value",
		Contacts: []string{
			"dats@chromium.org",
			"austinct@chromium.org",
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"drivefs",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		Pre: pre.DriveFsStarted,
		Vars: []string{
			"filemanager.user",
			"filemanager.password",
			"filemanager.drive_credentials",
		},
	})
}

func DrivefsSearch(ctx context.Context, s *testing.State) {
	APIClient := s.PreValue().(drivefs.PreData).APIClient
	tconn := s.PreValue().(drivefs.PreData).TestAPIConn

	// Create a file in Drive with time and random integer and defer the files removal.
	fileName := fmt.Sprintf("exact-file-drivefs-search-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
	file, err := APIClient.CreateBlankFileWithMimeType(ctx, fileName, "image/png", []string{"root"})
	if err != nil {
		s.Fatal("Failed creating a file in drive: ", err)
	}
	defer func() {
		// Log error when it fails but don't fail the overall test.
		if err := APIClient.RemoveFileByID(ctx, file.Id); err != nil {
			s.Logf("Failed cleaning up the file %q: %v", fileName, err)
		}
	}()
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Launch Files App
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not launch the Files App: ", err)
	}
	defer filesApp.Release(ctx)

	// Navigate to Google Drive via the Files App ui.
	if err := filesApp.OpenDrive(ctx); err != nil {
		s.Fatal("Could not open Google Drive folder: ", err)
	}

	// Get a keyboard handle to type into search box.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed trying to get the keyboard handle: ", err)
	}
	defer kb.Close()

	// Searches files app for the search term passed in.
	// Do not have to wait for the file to sync as search happens on remote, not locally.
	if err := filesApp.Search(ctx, kb, fileName); err != nil {
		s.Fatalf("Failed search for %q: %v", fileName, err)
	}

	// Get all the containers that contain the file names.
	params := ui.FindParams{
		ClassName: "filename-label",
		Role:      ui.RoleTypeGenericContainer,
	}
	resultsTextContainer, err := filesApp.Root.Descendants(ctx, params)
	if err != nil {
		s.Fatal("Failed getting all results in search container: ", err)
	}

	if len(resultsTextContainer) != 1 {
		s.Fatalf("Failed search identified %d results, expected 1", len(resultsTextContainer))
	}

	// Get the actual text from the container.
	text, err := resultsTextContainer[0].Descendant(ctx, ui.FindParams{Role: ui.RoleTypeStaticText})
	if err != nil {
		s.Fatal("Failed finding descendant text box: ", err)
	}

	if text.Name != fileName {
		s.Fatalf("Failed as search result is incorrect, expected %q, got %q", fileName, text.Name)
	}
}
