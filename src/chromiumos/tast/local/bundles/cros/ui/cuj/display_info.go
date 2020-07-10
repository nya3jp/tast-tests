// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
)

// AddDisplayInfoValue adds display info to performance metric values.
func AddDisplayInfoValue(ctx context.Context, tconn *chrome.TestConn, values *perf.Values) error {
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.New("failed to get display info")
	}
	if len(infos) == 0 {
		return errors.New("no display found")
	}
	score := 0.0
	for _, info := range infos {
		mode, err := info.GetSelectedMode()
		if err != nil {
			return errors.New("failed to get selected mode")
		}
		height, width, refreshRate := mode.Height, mode.Width, mode.RefreshRate
		score += float64(height*width) * refreshRate / 10000000
	}
	values.Set(perf.Metric{
		Name:      "TPS.Display",
		Unit:      "rate",
		Direction: perf.BiggerIsBetter,
		Multiple:  false,
	}, score/float64(len(infos)))

	return nil
}