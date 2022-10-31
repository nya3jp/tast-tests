// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/testing"
)

// Human readable strings.
const (
	chooseFromFileButtonName = "Choose a file"
	googleDrive              = "Google Drive"
	newAvatarFileName        = "chromium.png"
	googleProfilePhoto       = "Google profile photo"
	existingPhotoFromText    = "Existing photo from"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SelectAvatarFromFile,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting avatar from local files or Google Drive",
		Contacts: []string{
			"pzliu@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ambient.username", "ambient.password"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Fixture:      "personalizationWithGaiaLogin",
	})
}

func selectFromGoogleDrive(ctx context.Context, ui *uiauto.Context) error {
	chooseFromFileButton := nodewith.Role(role.ListBoxOption).Name(chooseFromFileButtonName).HasClass("avatar-button-container")
	googleDriveText := nodewith.Role(role.StaticText).Name(googleDrive)
	newAvatarIcon := nodewith.Role(role.StaticText).Name(newAvatarFileName)
	OpenButton := nodewith.Role(role.Button).NameContaining("Open").HasClass("ok primary")
	selectedAvatarOption := nodewith.Role(role.ListBoxOption).NameContaining(existingPhotoFromText).HasClass("tast-selected-last-external-image")

	return uiauto.Combine("choose a file from Google Drive",
		ui.WaitUntilExists(chooseFromFileButton),
		ui.DoDefault(chooseFromFileButton),
		ui.WaitUntilExists(googleDriveText),
		ui.DoDefault(googleDriveText),
		ui.WaitUntilExists(newAvatarIcon),
		ui.DoDefault(newAvatarIcon),
		ui.WaitUntilExists(OpenButton),
		ui.LeftClick(OpenButton),
		ui.WaitUntilExists(selectedAvatarOption),
	)(ctx)
}

func selectProfileImage(ctx context.Context, ui *uiauto.Context) error {
	googleProfilePhotoContainer := nodewith.Role(role.ListBoxOption).Name(googleProfilePhoto).HasClass("image-container")
	selectedAvatarOption := nodewith.Role(role.ListBoxOption).NameContaining(googleProfilePhoto).HasClass("tast-selected-profile-image")

	return uiauto.Retry(3, uiauto.Combine("change avatar back to Google profile photo",
		ui.WithTimeout(time.Second).DoDefault(googleProfilePhotoContainer),
		ui.WithTimeout(time.Second).WaitUntilExists(selectedAvatarOption),
	))(ctx)
}

func SelectAvatarFromFile(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Force Chrome to be in clamshell mode to make sure it's possible to close
	// the personalization hub.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	// Open avatar subpage.
	if err := uiauto.Combine("open avatar subpage",
		personalization.OpenPersonalizationHub(ui),
		personalization.OpenAvatarSubpage(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to open avatar subpage: ", err)
	}

	// Click the choose-from-file icon.
	if err := selectFromGoogleDrive(ctx, ui); err != nil {
		s.Fatal("Failed to choose a file from Google Drive: ", err)
	}

	// Change the avatar back to Google profile image.
	if err := selectProfileImage(ctx, ui); err != nil {
		s.Fatal("Failed to change the avatar back: ", err)
	}
}
