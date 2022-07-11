// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package youtubeweb

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
)

var videoPlayerContainer = nodewith.Name("YouTube Video Player").Role(role.GenericContainer)

func (yw *youtubeWeb) WithTimeout(timeout time.Duration) *youtubeWeb {
	return &youtubeWeb{
		tconn: yw.tconn,
		ui:    yw.ui.WithTimeout(timeout),
		conn:  yw.conn,
	}
}

func (yw *youtubeWeb) Info(ctx context.Context, finder *nodewith.Finder) (*uiauto.NodeInfo, error) {
	return yw.ui.Info(ctx, finder.FinalAncestor(yw.browserRoot))
}

func (yw *youtubeWeb) IsNodeFound(ctx context.Context, finder *nodewith.Finder) (bool, error) {
	return yw.ui.IsNodeFound(ctx, finder.FinalAncestor(yw.browserRoot))
}

func (yw *youtubeWeb) LeftClick(finder *nodewith.Finder) uiauto.Action {
	return yw.ui.LeftClick(finder.FinalAncestor(yw.browserRoot))
}

func (yw *youtubeWeb) Location(ctx context.Context, finder *nodewith.Finder) (*coords.Rect, error) {
	return yw.ui.Location(ctx, finder.FinalAncestor(yw.browserRoot))
}

func (yw *youtubeWeb) MouseMoveTo(finder *nodewith.Finder, duration time.Duration) uiauto.Action {
	return yw.ui.MouseMoveTo(finder.FinalAncestor(yw.browserRoot), duration)
}

func (yw *youtubeWeb) RetryUntil(action, condition func(context.Context) error) uiauto.Action {
	return yw.ui.RetryUntil(action, condition)
}

func (yw *youtubeWeb) WaitUntilExists(finder *nodewith.Finder) uiauto.Action {
	return yw.ui.WaitUntilExists(finder.FinalAncestor(yw.browserRoot))
}
