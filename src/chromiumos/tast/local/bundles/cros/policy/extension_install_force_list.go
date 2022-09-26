// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/pci"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtensionInstallForceList,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Behavior of ExtensionForceList policy",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
		},
		Attr:         []string{"group:commercial_limited"},
		VarDeps:      []string{"policy.ExtensionInstallForceList.username", "policy.ExtensionInstallForceList.password"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + time.Minute,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlagWithName("ExtensionInstallForceList", pci.VerifiedFunctionalityUI),
		},
	})
}

func ExtensionInstallForceList(ctx context.Context, s *testing.State) {
	// The user has the ExtensionInstallForceList policy set.
	username := s.RequiredVar("policy.ExtensionInstallForceList.username")
	password := s.RequiredVar("policy.ExtensionInstallForceList.password")

	cr, err := chrome.New(ctx, chrome.GAIALogin(chrome.Creds{User: username, Pass: password}), chrome.ProdPolicy())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

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

	// If the extension is installed, the Installed button will be present which is not clickable.
	installedButton := nodewith.Role(role.Button).Name("Installed")

	if err := uiauto.New(tconn).WithTimeout(15 * time.Second).WaitUntilExists(installedButton.First())(ctx); err != nil {
		s.Fatal("Finding button node failed: ", err)
	}
}
