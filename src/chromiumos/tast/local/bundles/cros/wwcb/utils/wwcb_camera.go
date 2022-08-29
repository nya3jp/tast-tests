// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// CameraGetColor get color from camera
func CameraGetColor(ctx context.Context, s *testing.State, cameraID string) (string, error) {
	want, err := GetPiColor(ctx, cameraID, "0")
	if err != nil {
		return "", errors.Wrap(err, "failed to execute GetPiColor")
	}
	return want, nil

}

// CameraCheckColor send request and compare with response
func CameraCheckColor(ctx context.Context, cameraID, wantColor string) error {
	color, err := GetPiColor(ctx, cameraID, "0")
	if err != nil {
		return errors.Wrap(err, "failed to execute GetPiColor")
	}

	if color != wantColor {
		return errors.Errorf("failed to get expect color; got %s, want %s", color, wantColor)
	}

	return nil
}

// CameraCheckColorLater just send request , let server know
func CameraCheckColorLater(ctx context.Context, s *testing.State, cameraID string) error {
	_, err := GetPiColor(ctx, cameraID, "5")
	if err != nil {
		return errors.Wrap(err, "failed to execute GetPiColor")
	}
	return nil
}

// CameraCheckColorResult send reuqest and compare with response
func CameraCheckColorResult(ctx context.Context, wantColor string) error {
	color, err := GetPiColorResult(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to execute GetPiColorResult")
	}
	if color != wantColor {
		return errors.Errorf("failed to get expect color, got %s, want %s", color, wantColor)
	}
	return nil
}

// GetOutputPath notice: return's path is partail not completed
// Remove this ahead path as follow: "/home/oem/r91/chroot/tmp/tast/results"
// Newman do last path, then send to web api
// James would combine ahead & last path together
func GetOutputPath(ctx context.Context, s *testing.State) string {
	var want string

	var str = s.OutDir()
	var delimiter = "/"
	var parts = strings.Split(str, delimiter)

	// filter s.OutDir(), then get relative path in tast server results folder
	for _, part := range parts {
		if strings.Contains(part, "tast_out") {
			var array = strings.Split(part, ".")
			want = filepath.Join(array[1], "tests", s.TestName())
		}
	}

	testing.ContextLogf(ctx, "Get ouput path in tast server results folder - %s", want)

	return want
}

// CameraCheckPlayback Check the 1Khz video/audio playback by test fixture
func CameraCheckPlayback(ctx context.Context, cameraID string) error {

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	// send "f" to enter youtube full screen
	if err := kb.Accel(ctx, "f"); err != nil {
		return errors.Wrap(err, "failed to let youtube into full screen")
	}

	// send http request to api server
	// tell server to record video with camera fixture
	videoPath, err := VideoRecord(ctx, "60", cameraID)
	if err != nil {
		return errors.Wrap(err, "failed to video record")
	}

	testing.ContextLogf(ctx, "File path is %s", videoPath)
	// send "esc" to chromebook exit youtube full screen
	if err := kb.Accel(ctx, "esc"); err != nil {
		return errors.Wrap(err, "failed to let youtube exit full screen")
	}

	// compare video with sample
	if err := DetectVideo(ctx, videoPath); err != nil {
		return errors.Wrap(err, "failed to compare video with sample")
	}
	return nil
}
