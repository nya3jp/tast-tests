// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SmartDim,
		Desc:         "Check the SmartDim can make decision with ML Service",
		Contacts:     []string{"alanlxl@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "ml_service", "smartdim"},
	})
}

func SmartDim(ctx context.Context, s *testing.State) {
	const (
		dbusName            = "org.chromium.MlDecisionService"
		dbusPath            = dbus.ObjectPath("/org/chromium/MlDecisionService")
		dbusInterfaceMethod = "org.chromium.MlDecisionService.ShouldDeferScreenDim"

		histogramName = "MachineLearningService.SmartDimModel.ExecuteResult.Event"
		timeout       = 10 * time.Second
	)
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--external-metrics-collection-interval=1"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	_, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}

	call := func() {
		s.Log("Asking /org/chromium/MlDecisionService for Smart Dim decision")
		var state bool
		if err := obj.CallWithContext(ctx, dbusInterfaceMethod, 0).Store(&state); err != nil {
			s.Error("Failed to get Smart Dim decision: ", err)
		} else {
			s.Log("Smart Dim decision is ", state)
		}
	}

	call()
	s.Logf("Waiting for %v histogram", histogramName)
	h1, err := metrics.WaitForHistogram(ctx, cr, histogramName, timeout)
	if err != nil {
		s.Fatal("Failed to get histogram: ", err)
	}
	s.Log("Got histogram: ", h1)

	call()
	s.Logf("Waiting for %v histogram update", histogramName)
	h2, err := metrics.WaitForHistogramUpdate(ctx, cr, histogramName, h1, timeout)
	if err != nil {
		s.Fatal("Failed to get histogram update: ", err)
	}
	s.Log("Got histogram update: ", h2)
	expectedBucket := metrics.HistogramBucket{Min: 0, Max: 1, Count: 1}
	if len(h2.Buckets) != 1 || h2.Buckets[0] != expectedBucket {
		s.Fatal("h2 expected value [[0,1):1], but get ", h2)
	}
}
