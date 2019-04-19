// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelJPEG,
		Desc:         "Run Chrome jpeg_encode_accelerator_unittest",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeJPEG},
		Data:         []string{"bali_640x360_P420.yuv"},
	})
}

// EncodeAccelJPEG runs a set of HW JPEG encode tests, defined in
// jpeg_encode_accelerator_unittest.
func EncodeAccelJPEG(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// Execute the test binary.
	args := []string{logging.ChromeVmoduleFlag(),
		"--yuv_filenames=" + s.DataPath("bali_640x360_P420.yuv") + ":640x360"}
	const exec = "jpeg_encode_accelerator_unittest"
	if ts, err := bintest.Run(ctx, exec, args, s.OutDir()); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
	}
}
