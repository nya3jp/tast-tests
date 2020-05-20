// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vimeo

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/dom"
)

const vimeoPlayer = `div.vp-video-wrapper > div.vp-video > div > video`

// PlayVideo triggers vimeoPlayer.play().
func PlayVideo(ctx context.Context, conn *chrome.Conn) error {
	if err := dom.PlayElement(ctx, conn, vimeoPlayer); err != nil {
		return mtbferrors.New(mtbferrors.ChromeExeJs, err, "OpenAndPlayVideo")
	}
	return nil
}

// PauseVideo triggers vimeoPlayer.pause().
func PauseVideo(ctx context.Context, conn *chrome.Conn) error {
	if err := dom.PauseElement(ctx, conn, vimeoPlayer); err != nil {
		return mtbferrors.New(mtbferrors.VideoPauseFailed, err, "Vimeo")
	}
	return nil
}
