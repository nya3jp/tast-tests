// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OobePerf,
		Desc: "Test waits for out-of-box-experience (OOBE) Welcome screen and measures the time of the WebUI loading",
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
	})
}

func OobePerf(ctx context.Context, s *testing.State) {
	const (
		histogramName = "OOBE.WebUI.LoadTime.FirstRun"
	)
	r := perfutil.NewRunner(nil)
	r.RunMultiple(ctx, s, "OobePerf",
		func(ctx context.Context) ([]*metrics.Histogram, error) {
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
			// Wait for the WebUI load time histogram reported.
			hist, err := metrics.WaitForHistogram(ctx, tLoginConn, histogramName, time.Minute)
			return []*metrics.Histogram{hist}, err
		},
		perfutil.StoreAllWithHeuristics("Duration"))
}
