// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vimeo

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/mtbf/video/media"
	"chromiumos/tast/local/chrome"
)

// IsPlaying checks whether vimeo is playing the video.
func IsPlaying(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	return media.IsPlaying(ctx, conn, timeout, vimeoPlayer)
}

// IsPausing checks whether vimeo video play has paused.
func IsPausing(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	return media.IsPausing(ctx, conn, timeout, vimeoPlayer)
}
