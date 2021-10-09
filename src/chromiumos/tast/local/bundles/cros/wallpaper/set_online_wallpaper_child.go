// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wallpaper

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SetOnlineWallpaperChild,
		Desc: "Test setting online wallpapers in the new wallpaper app for a child user",
		Contacts: []string{
			"tobyhuang@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"unicorn.wallpaperCategory", "unicorn.wallpaperName"},
		Fixture:      "familyLinkUnicornLoginWallpaper",
		Timeout:      5 * time.Minute,
	})
}

func SetOnlineWallpaperChild(ctx context.Context, s *testing.State) {
	collection := s.RequiredVar("unicorn.wallpaperCategory")
	// TODO(b/200817707): Update unicorn.yaml with the new image name.
	image := s.RequiredVar("unicorn.wallpaperName") + " Digital Art by Leo Natsume"

	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := wallpaper.OpenWallpaperPicker(ui)(ctx); err != nil {
		s.Fatal("Failed to open wallpaper picker: ", err)
	}
	if err := wallpaper.SelectCollection(ui, collection)(ctx); err != nil {
		s.Fatalf("Failed to select collection %q: %v", collection, err)
	}
	if err := wallpaper.SelectImage(ui, image)(ctx); err != nil {
		s.Fatalf("Failed to select image %q: %v", image, err)
	}
	if err := ui.WaitUntilExists(nodewith.Name(fmt.Sprintf("Currently set %v", image)).Role(role.Heading))(ctx); err != nil {
		s.Fatalf("Failed to validate selected wallpaper %q: %v", image, err)
	}
}
