// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mediarecorder

import (
	"io/ioutil"
	"os"
	"testing"

	"chromiumos/tast/testutil"
)

// TestComputeNumFrames checks whether mediarecorder.TestComputeNumFrames returns correct
// number of frames for a given MKV video.
func TestComputeNumFrames(t *testing.T) {
	const correctFrameNum = 313
	videoBytes, err := ioutil.ReadFile("testdata/test_video.mkv")
	if err != nil {
		t.Error(err, "failed to read video file")
	}

	tempDir := testutil.TempDir(t)
	defer os.RemoveAll(tempDir)

	frameNum, err := computeNumFrames(videoBytes, tempDir)
	if err != nil {
		t.Error(err, "failed to compute number of frames")
	}
	if frameNum != correctFrameNum {
		t.Errorf("computed number of frames is %d, expected %d", frameNum, correctFrameNum)
	}
}
