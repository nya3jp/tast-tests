// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/media/caps"
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

// Human readable strings.
const (
	takeLoopVideo    = "Create a looping video"
	takePhoto        = "Take a photo"
	usePhoto         = "Use this photo"
	useLoopVideo     = "Use this video"
	defaultImageName = "Person daydreaming"
	defaultImageID   = "84"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SelectAvatarFromCamera,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting avatar from camera",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		Fixture:      "personalizationWithGaiaLogin",
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "photo",
				Val:  takePhoto,
			},
			{
				Name: "video",
				Val:  takeLoopVideo,
			},
		},
	})
}

// selectDefaultUserAvatar selects a default image as user avatar.
func selectDefaultUserAvatar(ctx context.Context, ui *uiauto.Context, imageName, imageID string) error {
	defaultAvatarOption := nodewith.Role(role.ListBoxOption).Name(imageName)
	selectedAvatarOption := nodewith.Role(role.ListBoxOption).HasClass(fmt.Sprintf("tast-selected-default-user-image-%v", imageID))

	if err := uiauto.Combine("select a default avatar and validate selected avatar",
		ui.WaitUntilExists(defaultAvatarOption),
		ui.LeftClick(defaultAvatarOption),
		ui.WaitUntilExists(selectedAvatarOption))(ctx); err != nil {
		return errors.Wrap(err, "failed to validate selected avatar")
	}
	return nil
}

// takePhotoOrVideoAsAvatar captures a photo or a looping video based on the mediaType and sets it as user avatar.
func takePhotoOrVideoAsAvatar(ctx context.Context, ui *uiauto.Context, mediaType string) error {
	mediaButtonOption := nodewith.Role(role.ListBoxOption).Name(mediaType).HasClass("avatar-button-container")
	takeMediaButton := nodewith.Role(role.Button).Name(mediaType).HasClass("primary")

	var useMediaButtonName string
	if mediaType == takePhoto {
		useMediaButtonName = usePhoto
	} else if mediaType == takeLoopVideo {
		useMediaButtonName = useLoopVideo
	} else {
		return errors.Errorf("invalid media type: %v", mediaType)
	}

	useMediaButton := nodewith.Role(role.Button).Name(useMediaButtonName).HasClass("primary")

	selectedAvatarOption := nodewith.Role(role.ListBoxOption).HasClass("tast-selected-last-external-image")

	if err := uiauto.Combine("take a photo/video from camera and set as user avatar",
		ui.WaitUntilExists(mediaButtonOption),
		ui.DoDefault(mediaButtonOption),
		ui.WaitUntilExists(takeMediaButton),
		ui.DoDefault(takeMediaButton),
		ui.WaitUntilExists(useMediaButton),
		ui.DoDefault(useMediaButton),
		ui.WaitUntilExists(selectedAvatarOption),
	)(ctx); err != nil {
		errors.Wrapf(err, "failed to %v as user avatar", mediaType)
	}

	return nil
}

func SelectAvatarFromCamera(ctx context.Context, s *testing.State) {
	mediaType := s.Param().(string)
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

	// Open avatar subpage.
	if err := uiauto.Combine("open avatar subpage",
		personalization.OpenPersonalizationHub(ui),
		personalization.OpenAvatarSubpage(ui),
	)(ctx); err != nil {
		s.Fatal("Failed to open avatar subpage: ", err)
	}

	// Select a default avatar.
	if err := selectDefaultUserAvatar(ctx, ui, defaultImageName, defaultImageID); err != nil {
		s.Fatalf("Failed to select default avatar %v: %v", defaultImageName, err)
	}

	// Take a photo and set it as avatar.
	if err := takePhotoOrVideoAsAvatar(ctx, ui, mediaType); err != nil {
		s.Fatalf("Failed to %v as avatar: %v", mediaType, err)
	}
}
