// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/local/wallpaper/constants"
	"chromiumos/tast/testing"
)

type testParams struct {
	darkModeEnabled bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetAutoDarkLightWallpaper,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting auto theme D/L wallpapers in the personalization hub app",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Fixture:      "personalizationWithClamshell",
		Params: []testing.Param{
			{
				Name: "dark",
				Val: testParams{
					darkModeEnabled: true,
				},
			},
			{
				Name: "light",
				Val: testParams{
					darkModeEnabled: false,
				},
			},
		},
	})
}

func SetAutoDarkLightWallpaper(ctx context.Context, s *testing.State) {
	darkModeEnabled := s.Param().(testParams).darkModeEnabled
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := uiauto.Combine("Enable auto mode",
		personalization.OpenPersonalizationHub(ui),
		personalization.ToggleAutoMode(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to enable auto mode: ", err)
	}

	if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.forceAutoThemeMode)`, darkModeEnabled); err != nil {
		s.Fatalf("Failed to force auto theme mode for testing, dark mode enabled - %v: %v", darkModeEnabled, err)
	}

	// Open Elements collection.
	if err := uiauto.Combine(fmt.Sprintf("Select wallpaper from %v collection", constants.ElementCollection),
		personalization.OpenWallpaperSubpage(ui),
		wallpaper.SelectCollection(ui, constants.ElementCollection),
		ui.WaitUntilExists(personalization.BreadcrumbNodeFinder(constants.ElementCollection)),
	)(ctx); err != nil {
		s.Fatalf("Failed to select collection %v : %v", constants.ElementCollection, err)
	}

	if err := selectDLWallpaper(ctx, ui, constants.ElementCollection); err != nil {
		s.Fatalf("Failed to select an image in %v collection as wallpaper: %v", constants.ElementCollection, err)
	}

	currentWallpaper, err := wallpaper.CurrentWallpaper(ctx, ui)
	if err != nil {
		s.Fatal("Failed to get current wallpaper name: ", err)
	}

	if darkModeEnabled && !strings.Contains(currentWallpaper, "Dark") {
		s.Fatal("Failed to set wallpaper to dark mode, current wallpaper: ", currentWallpaper)
	}

	if !darkModeEnabled && !strings.Contains(currentWallpaper, "Light") {
		s.Fatal("Failed to set wallpaper to light mode, current wallpaper: ", currentWallpaper)
	}
}

// selectDLWallpaper selects an image in a D/L collection and sets it as wallpaper.
func selectDLWallpaper(ctx context.Context, ui *uiauto.Context, collection string) error {
	imagesFinder := nodewith.Role(role.ListBoxOption).Ancestor(nodewith.Role(role.Main).Name(collection))

	images, err := ui.NodesInfo(ctx, imagesFinder)
	if err != nil {
		return errors.Wrapf(err, "failed to find images in %v collection", collection)
	}
	if len(images) < 8 {
		return errors.Errorf("at least 8 image options for %v collection expected", collection)
	}

	// Select 3rd image as wallpaper.
	selectedImage := images[2]
	if err := uiauto.Combine("set D/L wallpaper",
		ui.MouseClickAtLocation(0, selectedImage.Location.CenterPoint()),
		ui.WaitUntilExists(nodewith.Role(role.Heading).NameContaining(selectedImage.Name)),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to select Element image %v", selectedImage.Name)
	}
	return nil
}
