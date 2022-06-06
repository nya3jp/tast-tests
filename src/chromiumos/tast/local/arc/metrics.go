// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// AppKills holds the number of apps that have been killed, categorized by
// priority and component doing the killing.
type AppKills struct {
	Oom                 int `json:"oom"`
	LmkdForeground      int `json:"lmkdForeground"`
	LmkdPerceptible     int `json:"lmkdPerceptible"`
	LmkdCached          int `json:"lmkdCached"`
	PressureForeground  int `json:"pressureForeground"`
	PressurePerceptible int `json:"pressurePerceptible"`
	PressureCached      int `json:"pressureCached"`
}

// Subtract returns a new AppKills containing the increase in kill counts from
// other to this. Used to compute the number of kills between two
// measurements.
func (k *AppKills) Subtract(other *AppKills) *AppKills {
	return &AppKills{
		Oom:                 k.Oom - other.Oom,
		LmkdForeground:      k.LmkdForeground - other.LmkdForeground,
		LmkdPerceptible:     k.LmkdPerceptible - other.LmkdPerceptible,
		LmkdCached:          k.LmkdCached - other.LmkdCached,
		PressureForeground:  k.PressureForeground - other.PressureForeground,
		PressurePerceptible: k.PressurePerceptible - other.PressurePerceptible,
		PressureCached:      k.PressureCached - other.PressureCached,
	}
}

// Total returns the total number of app kills of all types.
func (k *AppKills) Total() int {
	return k.Oom + k.LmkdForeground + k.LmkdPerceptible + k.LmkdCached + k.PressureForeground + k.PressurePerceptible + k.PressureCached
}

// LogPerfMetrics logs a perf.Metric for every kill counter, and a total.
func (k *AppKills) LogPerfMetrics(p *perf.Values, suffix string) {
	p.Set(perf.Metric{Name: "arc_oom_kills" + suffix, Unit: "count", Direction: perf.SmallerIsBetter}, float64(k.Oom))
	p.Set(perf.Metric{Name: "arc_lmkd_foreground_kills" + suffix, Unit: "count", Direction: perf.SmallerIsBetter}, float64(k.LmkdForeground))
	p.Set(perf.Metric{Name: "arc_lmkd_perceptible_kills" + suffix, Unit: "count", Direction: perf.SmallerIsBetter}, float64(k.LmkdPerceptible))
	p.Set(perf.Metric{Name: "arc_lmkd_cached_kills" + suffix, Unit: "count", Direction: perf.SmallerIsBetter}, float64(k.LmkdCached))
	p.Set(perf.Metric{Name: "arc_pressure_foreground_kills" + suffix, Unit: "count", Direction: perf.SmallerIsBetter}, float64(k.PressureForeground))
	p.Set(perf.Metric{Name: "arc_pressure_perceptible_kills" + suffix, Unit: "count", Direction: perf.SmallerIsBetter}, float64(k.PressurePerceptible))
	p.Set(perf.Metric{Name: "arc_pressure_cached_kills" + suffix, Unit: "count", Direction: perf.SmallerIsBetter}, float64(k.PressureCached))
	p.Set(perf.Metric{Name: "arc_total_kills" + suffix, Unit: "count", Direction: perf.SmallerIsBetter}, float64(k.Total()))
}

// GetAppKills reports how many apps have been killed by Android's Low Memory
// Killer Demon or ArcProcessService or Android's Linux OOM Killer.
func GetAppKills(ctx context.Context, tconn *chrome.TestConn) (*AppKills, error) {
	var counts AppKills
	if err := tconn.Call(ctx, &counts, `tast.promisify(chrome.autotestPrivate.getArcAppKills)`); err != nil {
		return nil, errors.Wrap(err, "failed to run autotestPrivate.getArcAppKills")
	}
	return &counts, nil
}
