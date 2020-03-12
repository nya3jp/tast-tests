// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package youtube

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/mtbf/video/media"
	"chromiumos/tast/local/chrome"
)

// VerifyPlaying checks videoPlayer.currentTime for a period of time to see if currentTime is moving forward.
func VerifyPlaying(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	return media.VerifyPlayingElement(ctx, conn, timeout, VideoPlayer)
}

// VerifyPausing checks videoPlayer.currentTime is moving or not.
func VerifyPausing(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	return media.VerifyPausingElement(ctx, conn, timeout, VideoPlayer)
}

// VerifyPauseAndResume combines VerifyPlaying and VerifyPausing
func VerifyPauseAndResume(ctx context.Context, conn *chrome.Conn) (err error) {
	return media.VerifyPauseAndResumeElement(ctx, conn, VideoPlayer)
}

// VerifyRandomSeeking randomly moves videoPlayer.currentTime to see if onseeked event works properly.
func VerifyRandomSeeking(ctx context.Context, conn *chrome.Conn, numSeeks int) (err error) {
	return media.VerifyRandomSeekingElement(ctx, conn, numSeeks, VideoPlayer)
}

// VerifyFastForward does a fast forward and verify it's current time is correct.
func VerifyFastForward(ctx context.Context, conn *chrome.Conn) (err error) {
	return media.VerifyFastForwardElement(ctx, conn, VideoPlayer)
}

// VerifyFastRewind does a fast rewind and v erify it's current time is correct.
func VerifyFastRewind(ctx context.Context, conn *chrome.Conn) (err error) {
	return media.VerifyFastRewindElement(ctx, conn, VideoPlayer)
}
