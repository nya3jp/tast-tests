// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dailymotion

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/mtbf/video/media"
	"chromiumos/tast/local/chrome"
)

// VerifyPlaying by checking videoPlayer.currentTime every 1 second for a period of time to see if currentTime is moving forward.
func VerifyPlaying(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	return media.VerifyPlayingElement(ctx, conn, timeout, VideoPlayer)
}

// VerifyPausing by checking videoPlayer.currentTime is moving or not.
func VerifyPausing(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	return media.VerifyPausingElement(ctx, conn, timeout, VideoPlayer)
}

// VerifyPauseAndResume combine VerifyPlaying and VerifyPausing
func VerifyPauseAndResume(ctx context.Context, conn *chrome.Conn) (err error) {
	return media.VerifyPauseAndResumeElement(ctx, conn, VideoPlayer)
}

// VerifyRandomSeeking by randomly moving videoPlayer.currentTime to see if onseeked event works properly.
func VerifyRandomSeeking(ctx context.Context, conn *chrome.Conn, numSeeks int) error {
	return media.VerifyRandomSeekingElement(ctx, conn, numSeeks, VideoPlayer)
}
