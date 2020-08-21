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

// IsPlaying checks whether dailymotion is playing the video.
func IsPlaying(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	return media.IsPlayingWithoutSnapshot(ctx, conn, timeout, VideoPlayer)
}

// IsPausing checks whether dailymotion video play has paused.
func IsPausing(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	return media.IsPausing(ctx, conn, timeout, VideoPlayer)
}

// PauseAndResume pauses dailymotion video play, and the resumes it.
func PauseAndResume(ctx context.Context, conn *chrome.Conn) (err error) {
	return media.PauseAndResume(ctx, conn, VideoPlayer)
}

// RandomSeek does random seek to dailymotion video.
func RandomSeek(ctx context.Context, conn *chrome.Conn, numSeeks int) error {
	return media.RandomSeek(ctx, conn, numSeeks, VideoPlayer)
}
