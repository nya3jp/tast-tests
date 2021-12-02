// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

// walker type walks through exif fields.
type walker struct{}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIImageExif,
		Desc:         "Verifies captured imaging metadata information on EXIF, using userfacing camera",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera", "informational"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
	})
}

// CCAUIImageExif verifies captured image metadata information on EXIF.
func CCAUIImageExif(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()

	isUserFacing := func() bool {
		facing, err := app.GetFacing(ctx)
		if err != nil {
			s.Fatal("Failed to get facing: ", err)
		}
		return facing == cca.FacingFront
	}

	// Check whether user facing camera switched.
	if !isUserFacing() {
		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Switch camera failed: ", err)
		}
		if !isUserFacing() {
			s.Fatal("Failed to get user facing camera")
		}
	}
	fileInfo, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		s.Fatal("Failed to capture picture: ", err)
	}
	path, err := app.FilePathInSavedDir(ctx, fileInfo[0].Name())
	if err != nil {
		s.Fatal("Failed to get file path in saved path: ", err)
	}

	// Read meta data info from exif for the captured image.
	// Example Image length, width, model name, DateTime, etc,.
	file, err := os.Open(path)
	if err != nil {
		s.Fatal("Failed to open file: ", err)
	}
	defer file.Close()
	data, err := exif.Decode(file)
	if err != nil {
		s.Fatal("Failed to decode exif: ", err)
	}
	var w walker
	// Walk calls the Walk method of w with the name and tag for every non-nil EXIF field.
	// If w aborts the walk with an error, that error is returned.
	if err := data.Walk(w); err != nil {
		s.Fatal("Failed to read exif data of captured image: ", err)
	}
}

// Walk to traverse all the EXIF fields
func (w walker) Walk(name exif.FieldName, tag *tiff.Tag) error {
	return nil
}
