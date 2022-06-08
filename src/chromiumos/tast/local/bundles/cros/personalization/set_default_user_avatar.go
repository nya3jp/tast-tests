// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetDefaultUserAvatar,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting default user avatar in the personalization hub app",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

func SetDefaultUserAvatar(ctx context.Context, s *testing.State) {
	const (
		firstImageName  = "Person daydreaming"
		firstImageID    = "84"
		secondImageName = "Basketball"
		secondImageID   = "53"
	)

	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("open avatar subpage",
		personalization.OpenPersonalizationHub(ui),
		personalization.OpenAvatarSubpage(ui))(ctx); err != nil {
		s.Fatal("Failed to open avatar subpage: ", err)
	}

	if err := testDefaultUserAvatar(ctx, ui, firstImageName, firstImageID); err != nil {
		s.Fatalf("Failed to select default avatar - %v: %v", firstImageName, err)
	}

	if err := testDefaultUserAvatar(ctx, ui, secondImageName, secondImageID); err != nil {
		s.Fatalf("Failed to select default avatar - %v: %v", secondImageName, err)
	}
}

func testDefaultUserAvatar(ctx context.Context, ui *uiauto.Context, imageName, imageID string) error {
	avatarOption := nodewith.Role(role.ListBoxOption).Name(imageName)
	selectedAvatar := nodewith.HasClass(fmt.Sprintf("selected-default-user-image-%v", imageID)).NameContaining(imageName)

	if err := uiauto.Combine("select a default avatar and validate selected avatar",
		ui.WaitUntilExists(avatarOption),
		ui.LeftClick(avatarOption),
		ui.WaitUntilExists(selectedAvatar))(ctx); err != nil {
		return errors.Wrap(err, "failed to validate selected avatar")
	}
	return nil
}
