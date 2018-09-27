// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"chromiumos/tast/local/bundles/cros/video/common"
	"chromiumos/tast/local/bundles/cros/video/vda"
	"chromiumos/tast/testing"
)

func init() {
	const (
		video = "test-25fps.h264"
	)
	testing.AddTest(&testing.Test{
		Func:         VDATestH264,
		Desc:         "Run Chrome video_decode_accelerator_unittest with h264 input",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{common.H264HWDecoding},
		Data:         []string{video},
	})
}

func VDATestH264(s *testing.State) {
	const (
		video      = "test-25fps.h264"
		videoParam = "320:240:250:258:35:150:1"
	)

	vda.RunTest(s, video, videoParam, []string{})
}
