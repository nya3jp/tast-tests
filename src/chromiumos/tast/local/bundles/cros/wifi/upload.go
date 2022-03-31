// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Upload,
		Desc:         "Connect to WiFi and upload a file to Gmail",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Vars:         []string{"wifissid", "wifipassword", "ui.gaiaPoolDefault"},
		Params: []testing.Param{{
			Name:    "bronze",
			Val:     10,
			Timeout: 10 * time.Minute,
		}, {
			Name:    "silver",
			Val:     15,
			Timeout: 15 * time.Minute,
		}, {
			Name:    "gold",
			Val:     20,
			Timeout: 20 * time.Minute,
		}},
	})
}

// Upload uploads a file to Gmail after connecting to WiFi.
func Upload(ctx context.Context, s *testing.State) {
	testIter := s.Param().(int)
	cr, err := chrome.New(ctx, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// generating test file of 10 mb.
	testFileName := "testfile.txt"
	data := make([]byte, int(1e7), int(1e7))
	f, err := os.Create(filepath.Join(filesapp.DownloadPath, testFileName))
	if err != nil {
		s.Error("Failed to create file: ", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		s.Error("Failed to write data: ", err)
	}
	defer os.Remove(filepath.Join(filesapp.DownloadPath, testFileName))

	ssid := s.RequiredVar("wifissid")
	wifiPwd := s.RequiredVar("wifipassword")

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
	defer cancel()

	ethEnabled, err := manager.IsEnabled(ctx, shill.TechnologyEthernet)
	if err != nil {
		s.Fatal("Failed to check if ethernet is enabled: ", err)
	}
	if ethEnabled {
		enableFunc, err := manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet)
		if err != nil {
			s.Fatal("Failed to disable ethernet: ", err)
		}
		defer enableFunc(cleanupCtx)
	}

	var wifi *shill.WifiManager
	if wifi, err = shill.NewWifiManager(ctx, nil); err != nil {
		s.Fatal("Failed to create shill Wi-Fi manager: ", err)
	}
	// Ensure wi-fi is enabled.
	if err := wifi.Enable(ctx, true); err != nil {
		s.Fatal("Failed to enable Wi-Fi: ", err)
	}
	s.Log("Wi-Fi is enabled")
	if err := wifi.ConnectAP(ctx, ssid, wifiPwd); err != nil {
		s.Fatalf("Failed to connect Wi-Fi AP %s: %v", ssid, err)
	}
	s.Logf("Wi-Fi AP %s is connected", ssid)

	gmailURL := "https://www.gmail.com/"
	gConn, err := cr.NewConn(ctx, gmailURL)
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer gConn.Close()

	composeButton := nodewith.Name("Compose").Role(role.Button)
	composeDialog := nodewith.Name("Compose: New Message").Role(role.Dialog).First()
	attachFilesButton := nodewith.Name("Attach files").Role(role.PopUpButton).Ancestor(composeDialog)
	downloadsButton := nodewith.Name("Downloads").Role(role.TreeItem)
	fileNode := nodewith.Name(testFileName).Role(role.StaticText)
	openButton := nodewith.Name("Open").Role(role.Button)
	removeattachmentButton := nodewith.Name("Remove attachment").Role(role.Button).Ancestor(composeDialog)

	ui := uiauto.New(tconn)

	// Find the Compose button node.
	if err := ui.WaitUntilExists(composeButton)(ctx); err != nil {
		s.Fatal("Failed to find the Compose button: ", err)
	}

	if err := ui.LeftClick(composeButton)(ctx); err != nil {
		s.Fatal("Failed to click the Compose button: ", err)
	}

	for i := 1; i <= testIter; i++ {
		s.Logf("Iteration: %d/%d", i, testIter)
		if err := uiauto.Combine("compose message and attach files",
			ui.WaitUntilExists(composeDialog),
			ui.WaitUntilExists(attachFilesButton),
			ui.LeftClick(attachFilesButton),
			ui.WaitUntilExists(downloadsButton),
			ui.LeftClick(downloadsButton),
			ui.WaitUntilExists(fileNode),
			ui.LeftClick(fileNode),
			ui.WaitUntilExists(openButton),
			ui.LeftClick(openButton),
			ui.WithTimeout(20*time.Second).WaitUntilExists(removeattachmentButton),
			ui.LeftClick(removeattachmentButton),
		)(ctx); err != nil {
			s.Fatal("Failed to compose message and attach file: ", err)
		}
	}
}
