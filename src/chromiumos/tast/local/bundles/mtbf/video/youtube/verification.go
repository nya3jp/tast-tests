// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package youtube

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/video/media"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// IsPlaying checks whether youtube is playing the video.
func IsPlaying(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	return media.IsPlaying(ctx, conn, timeout, VideoPlayer)
}

// IsPausing checks whether youtube video play has paused.
func IsPausing(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	return media.IsPausing(ctx, conn, timeout, VideoPlayer)
}

// PauseAndResume pauses youtube video play, and the resumes it.
func PauseAndResume(ctx context.Context, conn *chrome.Conn) (err error) {
	return media.PauseAndResume(ctx, conn, VideoPlayer)
}

// RandomSeek does random seek to youtube video.
func RandomSeek(ctx context.Context, conn *chrome.Conn, numSeeks int) (err error) {
	return media.RandomSeek(ctx, conn, numSeeks, VideoPlayer)
}

// FastForward does a fast forward and verifies youtube video current time is correct.
func FastForward(ctx context.Context, conn *chrome.Conn) (err error) {
	return media.FastForward(ctx, conn, VideoPlayer)
}

// FastRewind does a fast rewind and verifies youtube video current time is correct.
func FastRewind(ctx context.Context, conn *chrome.Conn) (err error) {
	return media.FastRewind(ctx, conn, VideoPlayer)
}

// CheckFramedrops checks frame drops every second for a given duration.
// Error will be returned based on number of frames dropped.
func CheckFramedrops(ctx context.Context, conn *chrome.Conn, timeout time.Duration) error {
	currentTime, err := CurrentTime(ctx, conn)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoGetTime, err)
	}
	var (
		interval  time.Duration = 1 * time.Second
		totalTime time.Duration
	)

	totalFramedrops := 0
	for totalTime <= timeout {
		totalTime += interval
		if err = testing.Sleep(ctx, interval); err != nil {
			return err
		}

		framedrops, err := GetFrameDropsFromStatsForNerds(ctx, conn)
		if err != nil {
			return mtbferrors.New(mtbferrors.VideoGetFrmDrop, err)
		}
		totalFramedrops += framedrops
	}

	const (
		minorFrameDropThreshold  = 1
		majorFrameDropThreshold  = 2
		severeFrameDropThreshold = 24
	)
	if totalFramedrops >= severeFrameDropThreshold {
		return mtbferrors.New(mtbferrors.VideoSevereFramedrops, nil, currentTime, totalFramedrops, timeout.Seconds())
	} else if totalFramedrops >= majorFrameDropThreshold {
		return mtbferrors.New(mtbferrors.VideoMajorFramedrops, nil, currentTime, totalFramedrops, timeout.Seconds())
	} else if totalFramedrops >= minorFrameDropThreshold {
		return mtbferrors.New(mtbferrors.VideoMinorFramedrops, nil, currentTime, totalFramedrops, timeout.Seconds())
	}

	return nil
}
