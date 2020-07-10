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

// DisplayInfo stores display configurations.
type DisplayInfo struct {
	refreshRate float64
	height      int
	width       int
}

// NewDisplayInfo creates a DisplayInfo based on the current display mode. It assumes
// there is only one display connected.
func NewDisplayInfo(ctx context.Context, tconn *chrome.TestConn) (*DisplayInfo, error) {

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return nil, errors.New("failed to get display info")
	}
	if len(infos) != 1 {
		return nil, errors.New("failed to find unique display")
	}
	mode, err := infos[0].GetSelectedMode()
	if err != nil {
		return nil, errors.New("failed to get selected mode")
	}
	return &DisplayInfo{
		refreshRate: mode.RefreshRate,
		width:       mode.Width,
		height:      mode.Height,
	}, nil
}

// ToValues stores display information as performance metric values.
func (d DisplayInfo) ToValues(values *perf.Values) {
	values.Set(perf.Metric{
		Name:      "TPS.RefreshRate",
		Unit:      "hz",
		Direction: perf.BiggerIsBetter,
		Multiple:  false,
	}, d.refreshRate)
	values.Set(perf.Metric{
		Name:      "TPS.Height",
		Unit:      "pixel",
		Direction: perf.BiggerIsBetter,
		Multiple:  false,
	}, float64(d.height))
	values.Set(perf.Metric{
		Name:      "TPS.Width",
		Unit:      "pixel",
		Direction: perf.BiggerIsBetter,
		Multiple:  false,
	}, float64(d.width))
}
