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
		Func:         FilesAppContextMenuPinAndUnpin,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that pinning to Holding Space from the Files app works",
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

// FilesAppContextMenuPinAndUnpin tests the functionality of pinning files to
// Holding Space from the Files app.
func FilesAppContextMenuPinAndUnpin(ctx context.Context, s *testing.State) {
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

	const fileName = "test.txt"
	filePath := filepath.Join(filesapp.MyFilesPath, fileName)

	// Create our file, with appropriate permissions so we can delete later.
	if err := ioutil.WriteFile(filePath, []byte("Per aspera, ad astra"), 0644); err != nil {
		s.Fatalf("Creating file %q failed: %s", filePath, err)
	}
	defer os.Remove(filePath)

	uia := uiauto.New(tconn)

	// Pin and unpin the item using the context menu in the files app. Unpinning
	// the item should make the Tray disappear, since nothing else is pinned.
	if err := uiauto.Combine(
		"Pin and unpin item from shelf via Files app context menu",
		fsapp.ClickContextMenuItem(fileName, "Pin to shelf"),
		uia.LeftClick(holdingspace.FindTray()),
		uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(fileName)),
		fsapp.ClickContextMenuItem(fileName, "Unpin from shelf"),
		uia.WaitUntilGone(holdingspace.FindTray()),
		uia.EnsureGoneFor(holdingspace.FindTray(), time.Second),
	)(ctx); err != nil {
		s.Fatalf("Failed to pin and unpin item %q: %s", fileName, err)
	}

}
