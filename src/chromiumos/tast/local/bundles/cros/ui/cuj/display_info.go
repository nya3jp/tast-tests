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

// DisplayInfoSource is a perf.TimelineDatasource reporting the display information.
type DisplayInfoSource struct {
	name  string
	tconn *chrome.TestConn
}

// NewDisplayInfoSource creates a new instance of DisplayInfoSource with the
// given name.
func NewDisplayInfoSource(name string, tconn *chrome.TestConn) *DisplayInfoSource {
	return &DisplayInfoSource{name: name, tconn: tconn}
}

// Setup implements perf.TimelineDatasource.Setup.
func (s *DisplayInfoSource) Setup(ctx context.Context, prefix string) error {
	s.name = prefix + s.name
	return nil
}

// Start implements perf.TimelineDatasource.Start.
func (s *DisplayInfoSource) Start(ctx context.Context) error {
	return nil
}

// Snapshot implements perf.TimelineDatasource.Snapshot.
func (s *DisplayInfoSource) Snapshot(ctx context.Context, values *perf.Values) error {
	infos, err := display.GetInfo(ctx, s.tconn)
	if err != nil {
		return errors.New("failed to get display info")
	}
	if len(infos) != 0 {
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
	values.Append(perf.Metric{
		Name:      s.name,
		Unit:      "rate",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}, score/float64(len(infos)))

	return nil
}
