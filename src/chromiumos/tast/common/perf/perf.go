// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perf provides utilities to build a JSON file that can be uploaded to
// Chrome Performance Dashboard (https://chromeperf.appspot.com/).
//
// Measurements processed by this package are stored in
// tests/<test-name>/results-chart.json in the Tast results dir.  The data is
// typically read by the Autotest TKO parser. In order to have metrics
// uploaded, they have to be whitelisted here:
// src/third_party/autotest/files/tko/perf_upload/perf_dashboard_config.json
//
// Chrome Performance Dashboard docs can be found here:
// https://github.com/catapult-project/catapult/tree/master/dashboard
//
// Usage example:
//
//  pv := perf.NewValues()
//  pv.Set(perf.Metric{
//      Name:       "mytest_important_quantity"
//      Unit:       "gizmos"
//      Direction:  perf.BiggerIsBetter
//  }, 42)
//  if err := pv.Save(s.OutDir()); err != nil {
//      s.Error("Failed saving perf data: ", err)
//  }
package perf

import "chromiumos/tast/local/perf"

// DefaultVariantName is the default variant name treated specially by the dashboard.
const DefaultVariantName = perf.DefaultVariantName

// Direction indicates which direction of change (bigger or smaller) means improvement
// of a performance metric.
type Direction = perf.Direction

const (
	// SmallerIsBetter means the performance metric is considered improved when it decreases.
	SmallerIsBetter Direction = perf.SmallerIsBetter

	// BiggerIsBetter means the performance metric is considered improved when it increases.
	BiggerIsBetter Direction = perf.BiggerIsBetter
)

// Metric defines the schema of a performance metric.
type Metric = perf.Metric

// Values holds performance metric values.
type Values = perf.Values

// NewValues returns a new empty Values.
func NewValues() *Values {
	return perf.NewValues()
}

// Format describes the output format for perf data.
type Format = perf.Format

const (
	// Crosbolt is used for Chrome OS infra dashboards (go/crosbolt).
	Crosbolt Format = perf.Crosbolt
	// Chromeperf is used for Chrome OS infra dashboards (go/chromeperf).
	Chromeperf Format = perf.Chromeperf
)
