// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vda

import (
	"fmt"

	"chromiumos/tast/local/bundles/cros/video/common"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/testing"
)

const (
	// TODO(hiroh): fill a right path.
	chromeBinaryTestDir = "/tmp/"
)

func RunTest(s *testing.State, video string, videoDataParam string, extraParams []string) {
	defer common.DisableVideoLogs(EnableVideoLogs())
	defer faillog.SaveIfError(s)
	// TODO(hiroh) nuke_chrome?
	testParamList := []string{common.ChromeVmoduleFlag(), fmt.Sprintf("--test_video_data=%s:%s", video, videoDataParam)} + extraParams
	testParams := strings.Join(testParamList[:], " ")
	binaryTestPath := filepath.Join(chromeBinaryTestDir, "video_decode_accelerator_unittest")
	// TODO(hiroh): Run the test with testParams
}
