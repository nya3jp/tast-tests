// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FilesAppContextMenuPinAndUnpin,
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

// FilesAppContextMenuPinAndUnpin tests the functionality of pinning files to Holding Space
// from the Files app.
func FilesAppContextMenuPinAndUnpin(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatalf("Failed to log in to Chrome: %s", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatalf("Failed to create Test API connection: %s", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	fsapp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatalf("Could not open filesapp: %s", err)
	}

	const fileName = "test.txt"
	filePath := filepath.Join(filesapp.MyFilesPath, fileName)

	// Create our file, with appropriate permissions so we can delete later.
	if err := ioutil.WriteFile(filePath, []byte("Per aspera, ad astra"), 0644); err != nil {
		s.Fatalf("Creating file %s failed: %s", filePath, err)
	}
	defer os.Remove(filePath)

	// Pin the file to the shelf by using the context menu in the files app.
	if err := fsapp.ClickContextMenuItem(fileName, "Pin to shelf")(ctx); err != nil {
		s.Fatalf("Pinning file %s failed: %s", fileName, err)
	}

	uia := uiauto.New(tconn)

	// Make sure the file got pinned.
	if err := holdingspace.OpenBubble(ctx, uia); err != nil {
		s.Fatalf("Failed to open holding space bubble: %s", err)
	}

	// Unpinning the item should make the Tray disappear, since nothing else is pinned
	if err := uiauto.Combine("Unpin item via holding space context menu",
		uia.RightClick(holdingspace.FindPinnedFileChip(fileName)),
		uia.LeftClick(holdingspace.FindContextMenuItem("Unpin")),
		uia.WaitUntilGone(holdingspace.FindTray()),
		uia.EnsureGoneFor(holdingspace.FindTray(), time.Second),
	)(ctx); err != nil {
		s.Fatalf("Failed to unpin file: %s", err)
	}

}
