// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"time"

	"chromiumos/tast/local/bundles/cros/filemanager/pre"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DrivefsDssOffline,
		Desc: "Verify that making a Docs/Sheets/Slides file available offline through Files App works",
		Contacts: []string{
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
		Timeout: 5 * time.Minute,
		Pre:     pre.DriveFsWithDssPinning,
		VarDeps: []string{
			"filemanager.user",
			"filemanager.password",
			"filemanager.drive_credentials",
		},
	})
}

func installExtension(ctx context.Context, tconn *chrome.TestConn, url string) error {
	cr := s.PreValue().(drivefs.PreData).Chrome

	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to navigate to extension: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	ui := uiauto.New(s.PreValue().(drivefs.PreData).TestAPIConn)
	button := nodewith.Name("Add to Chrome").Role(role.Button).First()
	if err := uiauto.Combine(fmt.Sprintf("Install extension from %q if not already installed", url),
		ui.WaitUntilExists(nodewith.NameRegex(regexp.MustCompile("^(Add to|Remove from) Chrome$")).Role(role.Button).First()),
		ui.IfSuccessThen(ui.Exists(button), uiauto.Combine("",
			ui.LeftClick(button),
			ui.LeftClick(nodewith.Name("Add extension").Role(role.Button).Attribute("restriction", restriction.None)),
			ui.WaitUntilExists(nodewith.NameContaining("has been added to Chrome").First()),
		)),
	)(ctx); err != nil {
		return err
	}
	return nil
}

func DrivefsDssOffline(ctx context.Context, s *testing.State) {
	APIClient := s.PreValue().(drivefs.PreData).APIClient
	tconn := s.PreValue().(drivefs.PreData).TestAPIConn

	testDocFileName := fmt.Sprintf("doc-drivefs-%d-%d", time.Now().UnixNano(), rand.Intn(10000))

	// Create a blank Google doc in the root GDrive directory.
	file, err := APIClient.CreateBlankGoogleDoc(ctx, testDocFileName, []string{"root"})
	if err != nil {
		s.Fatal("Could not create blank google doc: ", err)
	}
	defer APIClient.RemoveFileByID(ctx, file.Id)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := installExtension(ctx, tconn, "https://chrome.google.com/webstore/detail/application-launcher-for/lmjegmlicamnimmfhcmpkclmigmmcbeh"); err != nil {
		s.Fatal("Failed to install Application Launcher for Drive extension: ", err)
	}
	if err := installExtension(ctx, tconn, "https://chrome.google.com/webstore/detail/google-docs-offline/ghbmnnjooekpmoecnnnilnnbdlolhkhi"); err != nil {
		s.Fatal("Failed to install Google Docs Offline extension: ", err)
	}

	// Launch Files App and check that Drive is accessible.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not launch the Files App: ", err)
	}

	testFileNameWithExt := fmt.Sprintf("%s.gdoc", testDocFileName)
	if err := uiauto.Combine(fmt.Sprintf("Make test file %q in Drive available offline", testFileNameWithExt),
		filesApp.OpenDrive(),
		filesApp.WaitForFile(testFileNameWithExt),
		filesApp.ToggleAvailableOfflineForFile(testFileNameWithExt),
		filesApp.SelectFile(testFileNameWithExt),
		filesApp.WaitUntilExists(nodewith.Name("Available offline").Role(role.ToggleButton).Attribute("checked", checked.True)),
	)(ctx); err != nil {
		s.Fatalf("Failed to make test file %q in Drive available offline: %v", testFileNameWithExt, err)
	}
}
