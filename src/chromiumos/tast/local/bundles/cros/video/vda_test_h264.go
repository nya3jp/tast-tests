// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"chromiumos/tast/local/bundles/cros/video/common"
	"chromiumos/tast/local/bundles/cros/video/vda"
	"chromiumos/tast/testing"
)

const (
	video      = "test-25fps.h264"
	videoParam = "320:240:250:258:35:150:1"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VdaTestH264,
		Desc:         "Run Chrome video_decode_accelerator_unittest with h264 input",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{H264HWDecoding},
		Data:         []string{video},
	})
}

func VdaTestH264(s *testing.State) {
	vda.RunTest(s, video, videoParam, []string{})
}
