// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/filemanager/pre"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
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

func DrivefsDssOffline(ctx context.Context, s *testing.State) {
	APIClient := s.PreValue().(drivefs.PreData).APIClient
	cr := s.PreValue().(drivefs.PreData).Chrome
	tconn := s.PreValue().(drivefs.PreData).TestAPIConn

	testDocFileName := fmt.Sprintf("doc-drivefs-%d-%d", time.Now().UnixNano(), rand.Intn(10000))

	// Create a blank Google doc in the root GDrive directory.
	file, err := APIClient.CreateBlankGoogleDoc(ctx, testDocFileName, []string{"root"})
	if err != nil {
		s.Fatal("Could not create blank google doc: ", err)
	}
	defer APIClient.RemoveFileByID(ctx, file.Id)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Ensure the required extensions are installed.
	// TODO(austinct): Figure out why these extensions aren't being installed by default in tast tests.
	docsOfflineName := "Google Docs Offline"
	docsOfflineURL := "https://chrome.google.com/webstore/detail/google-docs-offline/ghbmnnjooekpmoecnnnilnnbdlolhkhi"
	docsOfflineExt := cws.App{Name: docsOfflineName, URL: docsOfflineURL, InstalledTxt: "Remove from Chrome",
		AddTxt: "Add to Chrome", ConfirmTxt: "Add extension"}
	if err := cws.InstallApp(ctx, cr, tconn, docsOfflineExt); err != nil {
		s.Fatal("Failed to install Google Docs Offline extension: ", err)
	}

	proxyExtName := "Application Launcher For Drive (by Google)"
	proxyExtURL := "https://chrome.google.com/webstore/detail/application-launcher-for/lmjegmlicamnimmfhcmpkclmigmmcbeh"
	proxyExt := cws.App{Name: proxyExtName, URL: proxyExtURL, InstalledTxt: "Remove from Chrome",
		AddTxt: "Add to Chrome", ConfirmTxt: "Add extension"}
	if err := cws.InstallApp(ctx, cr, tconn, proxyExt); err != nil {
		s.Fatal("Failed to install Application Launcher For Drive extension: ", err)
	}

	// Launch Files App.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not launch the Files App: ", err)
	}

	// Try make the newly created Google doc available offline.
	testFileNameWithExt := fmt.Sprintf("%s.gdoc", testDocFileName)
	if err := uiauto.Combine(fmt.Sprintf("Make test file %q in Drive available offline", testFileNameWithExt),
		filesApp.OpenDrive(),
		filesApp.WaitForFile(testFileNameWithExt),
		filesApp.ToggleAvailableOfflineForFile(testFileNameWithExt),
		filesApp.LeftClick(nodewith.Name("Enable Offline").Role(role.Button)),
		filesApp.SelectFile(testFileNameWithExt),
	)(ctx); err != nil {
		s.Fatalf("Failed to make test file %q in Drive available offline: %v", testFileNameWithExt, err)
	}

	s.Log("Waiting for the Available offline toggle to stabilize")
	var previousNodeInfo *uiauto.NodeInfo
	var currentNodeInfo *uiauto.NodeInfo
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		currentNodeInfo, err = filesApp.Info(ctx, nodewith.Name("Available offline").Role(role.ToggleButton))
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get info of the Available offline toggle"))
		}
		if previousNodeInfo != nil && previousNodeInfo.Checked == currentNodeInfo.Checked {
			return nil
		}
		previousNodeInfo = currentNodeInfo
		return errors.New("the Available offline toggle did not stabilize")
	}, &testing.PollOptions{Interval: 500 * time.Millisecond}); err != nil {
		s.Fatal("Failed to wait for the Available offline toggle to stabilize")
	}

	if currentNodeInfo.Checked != checked.True {
		s.Fatalf("The test file %q in Drive was not made available offline", testFileNameWithExt)
	}
}
