// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"

	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/media/binsetup"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelJPEG,
		Desc:         "Run Chrome jpeg_decode_accelerator_unittest",
		Contacts:     []string{"henryhsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeJPEG},
		Data:         jpegTestFiles,
	})
}

// jpegTestFiles lists the files required by the JPEG decode accelerator test.
// TODO(crbug.com/986074): we should move the WebPs to a seperate tast test.
// For now we make the WebP tests run with the jpeg_decode_accelerator_unittest binary.
var jpegTestFiles = []string{
	"BlackAndWhite_criss-cross_pattern_2015x2015.webp",
	"peach_pi-1280x720.jpg",
	"peach_pi-40x23.jpg",
	"peach_pi-41x22.jpg",
	"peach_pi-41x23.jpg",
	"pixel-1280x720.jpg",
	"pixel-1280x720-grayscale.jpg",
	"pixel-1280x720-yuv420.jpg",
	"pixel-1280x720-yuv444.jpg",
	"RGB_noise_2015x2015.webp",
	"RGB_noise_large_pixels_115x115.webp",
	"RGB_noise_large_pixels_2015x2015.webp",
	"RGB_noise_large_pixels_4000x4000.webp",
	"solid_green_2015x2015.webp",
}

// DecodeAccelJPEG runs a set of HW JPEG decode tests, defined in
// jpeg_decode_accelerator_unittest.
func DecodeAccelJPEG(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// The JPEG decode test operates on all files in a single directory.
	// testing.State doesn't guarantee that all data files will be stored in the same
	// directory, so copy them to a temp dir.
	tempDir := binsetup.CreateTempDataDir(s, "DecodeAccelJPEG.tast.", jpegTestFiles)
	defer os.RemoveAll(tempDir)

	// Execute the test binary.
	args := []string{logging.ChromeVmoduleFlag(), "--test_data_path=" + tempDir + "/"}
	const exec = "jpeg_decode_accelerator_unittest"
	if ts, err := bintest.Run(ctx, exec, args, s.OutDir()); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
	}
}
