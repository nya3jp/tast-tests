// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package phonehub

import (
	"context"

	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/crossdevice/phonehub"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DownloadRecentPhoto,
		Desc: "Exercises toggling the Recent Photos feature and downloading a photo from a connected phone",
		Contacts: []string{
			"jasonsun@chromium.org",
			"chromeos-sw-engprod@google.com",
			"chromeos-cross-device-eng@google.com",
		},
		Attr:         []string{"group:cross-device"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceOnboardedAllFeatures",
	})
}

// DownloadRecentPhoto exercises toggling the Recent Photos feature and downloading a photo from a connected phone.
func DownloadRecentPhoto(ctx context.Context, s *testing.State) {
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice
	chrome := s.FixtValue().(*crossdevice.FixtData).Chrome
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	ui := uiauto.New(tconn)

	photoName, err := androidDevice.TakePhoto(ctx)
	if err != nil {
		s.Fatal("Failed to take a photo on the Android phone: ", err)
	}

	// Open Phone Hub and enable Recent Photos via the opt-in view.
	if err := phonehub.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Phone Hub: ", err)
	}
	if err := ui.LeftClick(phonehub.FindRecentPhotosOptInButton())(ctx); err != nil {
		s.Fatal("Failed to enable Recent Photos via the opt-in view: ", err)
	}

	// Download the newly taken photo to Tote.
	if err := phonehub.DownloadMostRecentPhoto(ctx, tconn); err != nil {
		s.Fatal("Failed to download the most recent photo: ", err)
	}
	if err := uiauto.Combine("view downloaded photo in the holding space tray",
		ui.LeftClick(holdingspace.FindTray()),
		ui.Exists(holdingspace.FindDownloadChip().Name(photoName).First()),
	)(ctx); err != nil {
		s.Fatal("Expected photo ", photoName, " is not displayed in the holding space tray: ", err)
	}

	// Hide Phone Hub and disable Recent Photos from the Settings page.
	if err := phonehub.Hide(ctx, tconn); err != nil {
		s.Fatal("Failed to open Phone Hub: ", err)
	}
	if err := phonehub.ToggleRecentPhotosSetting(ctx, tconn, chrome, false); err != nil {
		s.Fatal("Failed to disable Recent Photos: ", err)
	}

	// Open Phone Hub and verify that the Recent Photos opt-in view is displayed.
	if err := phonehub.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Phone Hub: ", err)
	}
	if err := ui.Exists(phonehub.FindRecentPhotosOptInButton())(ctx); err != nil {
		s.Fatal("Recent Photos opt-in view is not displayed: ", err)
	}
}
