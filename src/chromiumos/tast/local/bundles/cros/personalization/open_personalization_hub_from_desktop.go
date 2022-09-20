// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"math/rand"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenPersonalizationHubFromDesktop,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test opening personalization hub app by right clicking on desktop",
		Contacts: []string{
			"cowmoo@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func OpenPersonalizationHubFromDesktop(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithPollOpts(testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 3 * time.Second})

	// Right click a random pixel in the upper left until the drop down menu exists.
	openMenu := ui.RetryUntil(
		ui.MouseClickAtLocation(1, coords.Point{X: rand.Intn(200), Y: rand.Intn(200)}),
		ui.Exists(personalization.SetPersonalizationMenu),
	)

	// Left click the option to open personalization hub.
	clickMenu := ui.RetryUntil(
		ui.LeftClick(personalization.SetPersonalizationMenu),
		ui.Exists(personalization.PersonalizationHubWindow))

	if err := uiauto.Combine("open personalization hub from menu",
		openMenu,
		clickMenu,
		// Window opened.
		ui.WaitUntilExists(personalization.PersonalizationHubWindow),
		// Content of Personalization loaded.
		ui.WaitUntilExists(personalization.ChangeWallpaperButton),
	)(ctx); err != nil {
		s.Fatal("Failed to open wallpaper app from desktop: ", err)
	}
}
