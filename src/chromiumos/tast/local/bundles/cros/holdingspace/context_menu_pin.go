// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"os"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ContextMenuPin,
		Desc: "Checks that pinning to Holding Space from the Files app works",
		Contacts: []string{
			"angusmclean@chromium.org",
			"dmblack@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedInDisableSync(),
	})
}

// ContextMenuPin tests the functionality of pinning files to Holding Space
// from the Files app.
func ContextMenuPin(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	fsapp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not open filesapp: ", err)
	}
	const fileName = "test1.txt"

	filePath, err := holdingspace.CreateAndPinNewfile(ctx, tconn, fsapp, fileName)
	defer os.Remove(filePath)

	// Unpin the first item through the context menu in the files app.
	if err := fsapp.ClickContextMenuItem(fileName, "Unpin from shelf")(ctx); err != nil {
		s.Fatalf("Unpinning file %s failed: %s", filePath, err)
	}

	if err := holdingspace.WaitUntilNotPinned(ctx, tconn, fileName); err != nil {
		s.Fatalf("File %s did not unpin: %s", filePath, err)
	}
}
