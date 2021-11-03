// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmputils contains utility functions for wmp tests.
package wmputils

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

// FindDeskMiniViews returns a list of DeskMiniView nodes and verifies the number of nodes.
// TODO(crbug/1251558): use autotest api to get the number of desks instead.
func FindDeskMiniViews(ctx context.Context, ac *uiauto.Context, count int) ([]uiauto.NodeInfo, error) {
	deskMiniViews := nodewith.ClassName("DeskMiniView")
	deskMiniViewsInfo, err := ac.NodesInfo(ctx, deskMiniViews)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find all desk mini views")
	}
	if len(deskMiniViewsInfo) != count {
		return nil, errors.Errorf("expected %v desks, but got %v instead", count, len(deskMiniViewsInfo))
	}
	return deskMiniViewsInfo, nil
}
