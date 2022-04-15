// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"path/filepath"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const (
	// NoRoom means not joining google meet when running the test.
	NoRoom = 0
	// TwoRoomSize creates a conference room with 2 participants.
	TwoRoomSize = 2
	// SmallRoomSize creates a conference room with 5 participants.
	SmallRoomSize = 5
	// LargeRoomSize creates a conference room with 16 participants.
	LargeRoomSize = 16
	// ClassRoomSize creates a conference room with 38 participants.
	ClassRoomSize = 38
	// CameraVideo is a video file used as a fake camera for conference testing.
	// Video shows a real person talking to the camera. Using this video as the camera input,
	// the effect of switching the background can be observed on the conference page.
	// Zoom supports up to 1080p HD video (Business, Education, or Enterprise account only),
	// while Google Meet supports 720p. So here use 720p HD video as test video.
	CameraVideo = "720p_camera_video.mjpeg"
)

// TestParameters defines the test parameters for conference.
type TestParameters struct {
	// Size is the conf room size.
	Size int
	// Tier defines the test tier: basic, plus, or premium.
	Tier string
	// IsLacros defines the browser type is Lacros or not.
	IsLacros bool
}

// PushFileToTmpDir copies the data file to the DUT tmp path, returning its path on the DUT.
func PushFileToTmpDir(ctx context.Context, s *testing.State, dut *dut.DUT, fileName string) (string, error) {
	const tmpDir = "/tmp"
	remotePath := filepath.Join(tmpDir, fileName)
	testing.ContextLog(ctx, "Copy the file to remote data path: ", remotePath)
	if _, err := linuxssh.PutFiles(ctx, dut.Conn(), map[string]string{
		s.DataPath(fileName): remotePath,
	}, linuxssh.DereferenceSymlinks); err != nil {
		return "", errors.Wrapf(err, "failed to send data to remote data path %v", remotePath)
	}
	return remotePath, nil
}
