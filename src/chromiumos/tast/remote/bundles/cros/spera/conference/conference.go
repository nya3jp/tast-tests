// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package conference contains remote Tast tests which conference related.
package conference

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const (
	// CPUIdleTimeout is used to wait for the cpu idle.
	CPUIdleTimeout = 2 * time.Minute
	// CameraVideo is a video file used as a fake camera for conference testing.
	// Video shows a real person talking to the camera. Using this video as the camera input,
	// the effect of switching the background can be observed on the conference page.
	// Zoom supports up to 1080p HD video (Business, Education, or Enterprise account only),
	// while Google Meet supports 720p. So here use 720p HD video as test video.
	CameraVideo = "720p_camera_video.mjpeg"
	// TraceConfigFile is the data path of the trace config file in text proto format.
	TraceConfigFile = "perfetto/system_trace_config.pbtxt"
)

// TestParameters defines the test parameters for conference.
type TestParameters struct {
	// RoomType defines the conference room type.
	RoomType RoomType
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
