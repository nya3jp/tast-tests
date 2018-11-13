// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/chrometest"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelJPEG,
		Desc:         "Run Chrome jpeg_encode_accelerator_unittest",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeJPEG},
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
	args := []string{
		logging.ChromeVmoduleFlag(),
		"--yuv_filenames=" + s.DataPath("bali_640x360_P420.yuv") + ":640x360"}
	if err := chrometest.Run(ctx, s.OutDir(),
		"jpeg_encode_accelerator_unittest", args); err != nil {
		s.Fatal("Failed to run jpeg_encode_accelerator_unittest: ", err)
	}
}
