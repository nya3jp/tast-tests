// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package phonehub

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/crossdevice/phonehub"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DownloadRecentPhoto,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Exercises toggling the Recent Photos feature and downloading a photo from a connected phone",
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
	androidFilePath := filepath.Join(crossdevice.AndroidPhotosPath, photoName)
	defer androidDevice.RemoveMediaFile(ctx, androidFilePath)

	// Open Phone Hub and enable Recent Photos via the opt-in view.
	if err := phonehub.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Phone Hub: ", err)
	}
	if err := phonehub.OptInRecentPhotos(ctx, tconn, chrome); err != nil {
		s.Fatal("Failed to enable Recent Photos via the opt-in view: ", err)
	}
	if err := androidDevice.TurnOnRecentPhotosFeature(ctx); err != nil {
		s.Fatal("Failed to enable Recent Photos on the phone: ", err)
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

	// Verify the downloaded file content.
	sourceFileSizeBytes, err := androidDevice.FileSize(ctx, androidFilePath)
	if err != nil {
		s.Fatal("Failed to read source photo size: ", err)
	}
	if err := waitUntilDownloadComplete(ctx, photoName, sourceFileSizeBytes); err != nil {
		s.Fatal("Photo download cannot be completed: ", err)
	}
	if err := comparePhotoHashes(ctx, photoName, androidDevice); err != nil {
		s.Fatal("Failed to verify hash of the downloaded photo: ", err)
	}

	// Hide Phone Hub and disable Recent Photos from the Settings page.
	if err := phonehub.Hide(ctx, tconn); err != nil {
		s.Fatal("Failed to open Phone Hub: ", err)
	}
	if err := phonehub.ToggleRecentPhotosSetting(ctx, tconn, chrome, false); err != nil {
		s.Fatal("Failed to disable Recent Photos: ", err)
	}
}

// waitUntilDownloadComplete waits for the target photo to be fully downloaded to the CrOS device's download directory.
func waitUntilDownloadComplete(ctx context.Context, photoName string, sourceFileSizeBytes int64) error {
	crosFilePath := filepath.Join(filesapp.DownloadPath, photoName)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		fi, err := os.Stat(crosFilePath)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get the size of the downloaded photo on CrOS"))
		}
		if currentSizeBytes := fi.Size(); currentSizeBytes != sourceFileSizeBytes {
			return errors.Errorf("Photo download not complete yet: expected %d bytes, downloaded %d bytes", sourceFileSizeBytes, currentSizeBytes)
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait for photo download to complete")
	}
	return nil
}

// comparePhotoHashes verifies that the hash of the downloaded photo matches the hash of the source photo on the Android device.
func comparePhotoHashes(ctx context.Context, photoName string, androidDevice *crossdevice.AndroidDevice) error {
	androidFilePath := filepath.Join(crossdevice.AndroidPhotosPath, photoName)
	androidHash, err := androidDevice.SHA256Sum(ctx, androidFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to compute hash of the source photo on the Android device")
	}

	crosFilePath := filepath.Join(filesapp.DownloadPath, photoName)
	crosHash, err := hashFile(ctx, crosFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to compute hash of the downloaded photo on the CrOS device")
	}

	if androidHash != crosHash {
		return errors.Errorf("Hash mismatch for downloaded photo %s: expected=%s, actual=%s", photoName, androidHash, crosHash)
	}

	return nil
}

// hashFile computes the SHA256 checksum of the given file.
func hashFile(ctx context.Context, filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open file %s", filePath)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", errors.Wrapf(err, "failed to copy %s file contents to the hasher", filePath)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
