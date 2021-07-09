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
		Desc: "Measures time of loading OOBE WebUI",
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
			cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
			defer cr.Close(ctx)
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "creating login test api connection failed")
			}
			hist, err := metrics.WaitForHistogram(ctx, tLoginConn, histogramName, time.Minute)
			return []*metrics.Histogram{hist}, err
		},
		perfutil.StoreAllWithHeuristics("Duration"))
}
