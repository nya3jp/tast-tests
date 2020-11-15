// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ExtensionInstallForceList,
		Desc: "Behavior of ExtensionForceList policy",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"policy.ExtensionInstallForceList.username", "policy.ExtensionInstallForceList.password"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
	})
}

func ExtensionInstallForceList(ctx context.Context, s *testing.State) {
	const (
		cleanupTime = 10 * time.Second // time reserved for cleanup.
	)

	// Use a shorter context to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	// The user has the ExtensionInstallForceList policy set.
	username := s.RequiredVar("policy.ExtensionInstallForceList.username")
	password := s.RequiredVar("policy.ExtensionInstallForceList.password")

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	cr, err := chrome.New(ctx, chrome.Auth(username, password, ""), chrome.GAIALogin(), chrome.ProdPolicy())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	const (
		extensionID = "hoppbgdeajkagempifacalpdapphfoai"
		downloadURL = "https://chrome.google.com/webstore/detail/platformkeys-test-extensi/" + extensionID
	)

	sconn, err := cr.NewConn(ctx, downloadURL)
	if err != nil {
		s.Fatal("Failed to connect to the extension page: ", err)
	}
	defer sconn.Close()

	// If the extension is installed, the Remove button will be present.
	removeButtonParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Remove from Chrome",
	}
	// Find the toggle button node.
	node, err := ui.FindWithTimeout(ctx, tconn, removeButtonParams, 15*time.Second)
	if err != nil {
		s.Fatal("Finding button node failed: ", err)
	}
	defer node.Release(cleanupCtx)

	// Try clicking the Remove from Chrome button.
	// We need to focus the button first so it will be clickable.
	if err := node.FocusAndWait(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to call focus() on the Advanced button: ", err)
	}

	if err := node.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the Remove from Chrome button: ", err)
	}

	// The extension should not be removed after clicking on the button as it is force installed.
	// Find the button node.
	node, err = ui.FindWithTimeout(ctx, tconn, removeButtonParams, 15*time.Second)
	if err != nil {
		s.Fatal("Finding button node failed: ", err)
	}
	defer node.Release(cleanupCtx)
}
