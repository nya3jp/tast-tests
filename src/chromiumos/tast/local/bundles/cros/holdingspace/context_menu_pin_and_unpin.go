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
		Func: ContextMenuPinAndUnpin,
		Desc: "Checks that pinning to Holding Space from the Files app works",
		Contacts: []string{
			"angusmclean@chromium.org",
			"dmblack@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// ContextMenuPinAndUnpin tests the functionality of pinning files to Holding Space
// from the Files app.
func ContextMenuPinAndUnpin(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in to Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	fsapp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not open filesapp: ", err)
	}

	junkFile, err := holdingspace.ForceShowHoldingSpaceTrayIcon(ctx, tconn, fsapp)
	if err != nil {
		s.Fatalf("Junk file creation failed: %s", err)
	}
	defer os.Remove(junkFile)

	const fileName = "test.txt"
	filePath, err := holdingspace.CreateFile(ctx, tconn, fileName)

	if err != nil {
		s.Fatalf("Failed to create file: %s", err)
	}
	defer os.Remove(filePath)

	if err := holdingspace.PinViaFileContextMenu(ctx, tconn, fsapp, fileName); err != nil {
		s.Fatalf("Failed to pin file: %s", err)
	}

	if err := holdingspace.UnpinItemViaContextMenu(ctx, tconn, fileName); err != nil {
		s.Fatalf("Failed to unpin file: %s", err)
	}
}
