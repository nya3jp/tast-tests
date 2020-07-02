// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"chromiumos/tast/common/perf"
)

// TestMetrics returns a slice of metrics that should be used for power
// tests.
func TestMetrics() []perf.TimelineDatasource {
	return []perf.TimelineDatasource{
		NewRAPLMetrics(),
		NewRAPLPowerMetrics(),
		NewSysfsBatteryMetrics(),
		NewSysfsThermalMetrics(),
	}
}

// TestMetricsWithoutBattery returns a slice of metrics that should be
// used for power tests on DUTs without a battery
func TestMetricsWithoutBattery() []perf.TimelineDatasource {
	return []perf.TimelineDatasource{
		NewRAPLMetrics(),
		NewRAPLPowerMetrics(),
		NewSysfsThermalMetrics(),
	}
}
