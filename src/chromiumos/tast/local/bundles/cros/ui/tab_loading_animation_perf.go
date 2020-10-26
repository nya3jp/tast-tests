// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabLoadingAnimationPerf,
		Desc:         "Measures the animation smoothness of tab loading animation",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          chrome.LoggedIn(),
		Timeout:      4 * time.Minute,
		Data: []string{
			"tab_loading_test.html",
		},
	})
}

func TabLoadingAnimationPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		conn, err := cr.NewConn(ctx, server.URL+"/tab_loading_test.html")
		if err != nil {
			s.Fatal("Failed to open a testing page: ", err)
		}
		defer conn.Close()
		defer conn.CloseTarget(ctx)
		return nil
	},
		"Chrome.Tabs.AnimationSmoothness.TabLoading"),
		perfutil.StoreSmoothness)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
