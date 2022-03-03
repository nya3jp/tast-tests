// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// DisplaySequence sequence number for display.
type DisplaySequence int

// DisplaySequence available display option
const (
	InternalDisplay  DisplaySequence = 0
	ExternalDisplay1 DisplaySequence = 1
	ExternalDisplay2 DisplaySequence = 2
)

// getDispTypeAndIndex return correspond tpye and index
func getDispTypeAndIndex(display DisplaySequence) (string, string) {
	switch display {
	case ExternalDisplay1:
		return ExtDisp1Type, ExtDisp1Index
	case ExternalDisplay2:
		return ExtDisp2Type, ExtDisp2Index
	default: // default is internal
		return IntDispType, IntDispIndex
	}
}

// GetColor get color from camera
func GetColor(ctx context.Context, s *testing.State, dispSeq DisplaySequence) (string, error) {

	dispType, dispIndex := getDispTypeAndIndex(dispSeq)

	want, err := GetPiColor(s, dispType, dispIndex, "0")
	if err != nil {
		return "", errors.Wrap(err, "failed to execute GetPiColor")
	}

	return want, nil

}

// CheckColor send request and compare with response
func CheckColor(ctx context.Context, s *testing.State, dispSeq DisplaySequence, expectColor string) error {

	dispType, dispIndex := getDispTypeAndIndex(dispSeq)

	color, err := GetPiColor(s, dispType, dispIndex, "0")
	if err != nil {
		return errors.Wrap(err, "failed to execute GetPiColor")
	}

	if color != expectColor {
		return errors.Errorf("failed to get expect color, got %s, want %s", color, expectColor)
	}

	return nil
}

// CheckColorLater just send request , let server know
func CheckColorLater(s *testing.State, dispSeq DisplaySequence) error {

	dispType, dispIndex := getDispTypeAndIndex(dispSeq)

	_, err := GetPiColor(s, dispType, dispIndex, "5")
	if err != nil {
		return errors.Wrap(err, "failed to execute GetPiColor")
	}

	return nil
}

// CheckColorResult send reuqest and compare with response
func CheckColorResult(s *testing.State, expectColor string) error {

	color, err := GetPiColorResult(s)
	if err != nil {
		return errors.Wrap(err, "failed to execute GetPiColorResult")
	}

	if color != expectColor {
		return errors.Errorf("failed to get expect color, got %s, want %s", color, expectColor)
	}

	return nil
}

// GetOutputPath notice: return's path is partail not completed
func GetOutputPath(s *testing.State) string {

	// Remove this ahead path as follow: "/home/oem/r91/chroot/tmp/tast/results"
	// Newman do last path, then send to web api
	// James would combine ahead & last path together

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

	s.Logf("Get ouput path in tast server results folder - %s", want)

	return want
}

// CheckPlaybackByFixture Check the 1Khz video/audio playback by test fixture
func CheckPlaybackByFixture(ctx context.Context, s *testing.State, dispSeq DisplaySequence) error {

	dispType, dispIndex := getDispTypeAndIndex(dispSeq)

	// get solid output path
	folderPath := GetOutputPath(s)

	testing.Sleep(ctx, 1*time.Second)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// send "f" to enter youtube full screen
	if err := kb.Accel(ctx, "f"); err != nil {
		return errors.Wrap(err, "failed to let youtube into full screen")
	}

	// send http request to api server
	// tell server to record video with camera fixture
	videoPath, err := VideoRecord(s, "60", folderPath, dispType, dispIndex)
	if err != nil {
		s.Fatal("Failed to capture video: ", err)
	} else {
		s.Logf("File path is %s", videoPath)
	}

	// send "esc" to chromebook exit youtube full screen
	if err := kb.Accel(ctx, "esc"); err != nil {
		return errors.Wrap(err, "failed to let youtube exit full screen")
	}

	// this is golden sample path
	// videoPath = "/home/oem/tast20.04/goldenSample/golden_sample.mp4"

	// compare video with golden sample
	if err := GoldenPredict(s, videoPath, dispType, dispIndex, false); err != nil {
		s.Fatal("Failed to check video: ", err)
	}

	return nil
}
