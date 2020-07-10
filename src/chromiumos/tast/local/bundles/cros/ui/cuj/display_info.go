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

	// TODO(yichenz): Consider about the multiple displays.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the primary display info")
	}
	mode, err := info.GetSelectedMode()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get selected mode")
	}
	return &DisplayInfo{
		refreshRate: mode.RefreshRate,
		width:       mode.Width,
		height:      mode.Height,
	}, nil
}

// ToValues stores display information as performance metric values.
func (d DisplayInfo) Record(values *perf.Values) {
	values.Set(perf.Metric{
		Name:      "TPS.RefreshRate",
		Unit:      "Hz",
		Direction: perf.BiggerIsBetter,
		Multiple:  false,
	}, d.refreshRate)
	values.Set(perf.Metric{
		Name:      "TPS.Height",
		Unit:      "Pixel",
		Direction: perf.BiggerIsBetter,
		Multiple:  false,
	}, float64(d.height))
	values.Set(perf.Metric{
		Name:      "TPS.Width",
		Unit:      "Pixel",
		Direction: perf.BiggerIsBetter,
		Multiple:  false,
	}, float64(d.width))
}
