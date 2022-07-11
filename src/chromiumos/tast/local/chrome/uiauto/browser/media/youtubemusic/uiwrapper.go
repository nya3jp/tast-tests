// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package youtubemusic

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

var (
	videoPlayerContainer = nodewith.Name("YouTube Video Player").Role(role.GenericContainer)
	playerBar            = nodewith.HasClass("left-controls style-scope ytmusic-player-bar").Role(role.GenericContainer)
	playPauseButton      = nodewith.Role(role.Button).HasClass("play-pause-button style-scope ytmusic-player-bar").Ancestor(playerBar)
)

func (ytm *youtubeMusic) WithTimeout(timeout time.Duration) *youtubeMusic {
	return &youtubeMusic{
		ui:   ytm.ui.WithTimeout(timeout),
		conn: ytm.conn,
	}
}

func (ytm *youtubeMusic) Info(ctx context.Context, finder *nodewith.Finder) (*uiauto.NodeInfo, error) {
	return ytm.ui.Info(ctx, finder.FinalAncestor(ytm.browserRoot))
}

func (ytm *youtubeMusic) LeftClick(finder *nodewith.Finder) uiauto.Action {
	return ytm.ui.LeftClick(finder.FinalAncestor(ytm.browserRoot))
}

func (ytm *youtubeMusic) WaitUntilExists(finder *nodewith.Finder) uiauto.Action {
	return ytm.ui.WaitUntilExists(finder.FinalAncestor(ytm.browserRoot))
}

func (ytm *youtubeMusic) WaitUntilGone(finder *nodewith.Finder) uiauto.Action {
	return ytm.ui.WaitUntilGone(finder.FinalAncestor(ytm.browserRoot))
}
