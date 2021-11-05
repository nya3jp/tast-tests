// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
)

// FrameDataSource is a perf.TimelineDatasource to collect smoothness of chrome
// ui::Compositor and record it under a given name.
type FrameDataSource struct {
	tconn *chrome.TestConn
	name  string
}

// NewFrameDataSource creates an instance of FrameDataSource.
func NewFrameDataSource(tconn *chrome.TestConn, name string) *FrameDataSource {
	return &FrameDataSource{
		tconn: tconn,
		name:  name,
	}
}

// Setup implements perf.TimelineDatasource.Setup.
func (s *FrameDataSource) Setup(ctx context.Context, prefix string) error {
	s.name = prefix + s.name
	return nil
}

// Start implements perf.TimelineDatasource.Start.
func (s *FrameDataSource) Start(ctx context.Context) error {
	return nil
}

// Snapshot implements perf.TimelineDatasource.Snapshot.
func (s *FrameDataSource) Snapshot(ctx context.Context, values *perf.Values) error {
	var smoothness float64
	if err := s.tconn.Call(ctx, &smoothness, `tast.promisify(chrome.autotestPrivate.getDisplaySmoothness)`); err != nil {
		return err
	}

	values.Append(perf.Metric{
		Name:      s.name,
		Multiple:  true,
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, smoothness)

	return nil
}

// Stop does nothing.
func (s *FrameDataSource) Stop(_ context.Context, values *perf.Values) error {
	return nil
}
