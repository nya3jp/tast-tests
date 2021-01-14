// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/camera/chart"
	"chromiumos/tast/testing"
)

// chartPre implements testing.Precondition.
type chartPre struct {
	// name is the name of chart.
	name string
	// path is the chart path in source tree.
	path string
	// chart controls chart display on chart tablet. Sets to nil if chart
	// is not ready.
	chart *chart.Chart
}

func newDataChartPre(name, path string) *chartPre {
	return &chartPre{name: name, path: path}
}

var dataChartScene = newDataChartPre("cts_portrait_scene", "third_party/cts_portrait_scene.jpg")

// DataChartScene returns test precondition for displaying default test scene on chart tablet.
func DataChartScene() *chartPre {
	return dataChartScene
}

func (p *chartPre) String() string         { return "chart_" + p.name }
func (p *chartPre) Timeout() time.Duration { return 2 * time.Minute }
func (p *chartPre) DataPath() string       { return p.path }

func (p *chartPre) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	if p.chart == nil {
		var altHostname string
		if hostname, ok := s.Var("chart"); ok {
			altHostname = hostname
		}
		c, err := chart.New(ctx, s.DUT(), altHostname, s.DataPath(p.path), s.OutDir())
		if err != nil {
			s.Fatal("Failed to prepare chart tablet: ", err)
		}
		p.chart = c
	}
	return nil
}

func (p *chartPre) Close(ctx context.Context, s *testing.PreState) {
	if p.chart == nil {
		return
	}
	if err := p.chart.Close(ctx, s.OutDir()); err != nil {
		s.Error("Failed to cleanup chart: ", err)
	}
}
