// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobePerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test waits for out-of-box-experience (OOBE) Welcome screen and measures the time of the WebUI loading",
		Contacts: []string{
			"rsorokin@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
		},
		Attr: []string{"group:crosbolt", "crosbolt_perbuild"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
	})
}

func OobePerf(ctx context.Context, s *testing.State) {
	const (
		histogramName = "OOBE.WebUI.LoadTime.FirstRun"
	)
	r := perfutil.NewRunner(nil)
	r.RunMultiple(ctx, "OobePerf", uiperf.Run(s, func(ctx context.Context, name string) ([]*metrics.Histogram, error) {
		// Load OOBE Welcome Screen (first OOBE screen). Test extension is required to fetch histograms.
		cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}

		defer cr.Close(ctx)

		tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "creating login test api connection failed")
		}
		// Wait for the WebUI load time histogram reported. 10 seconds should be enough even on the slowest boards. Making it 15 just in case.
		hist, err := metrics.WaitForHistogram(ctx, tLoginConn, histogramName, 15*time.Second)
		return []*metrics.Histogram{hist}, err
	}),
		perfutil.StoreAllWithHeuristics("Duration"))

	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed to save performance data in file: ", err)
	}
}
