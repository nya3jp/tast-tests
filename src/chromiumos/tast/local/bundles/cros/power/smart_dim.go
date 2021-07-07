// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SmartDim,
		Desc:         "Check the SmartDim can make decision with ML Service",
		Contacts:     []string{"alanlxl@chromium.org"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "ml_service", "smartdim"},
		Params: []testing.Param{{
			Val:     lacros.ChromeTypeChromeOS,
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "lacros",
			Val:               lacros.ChromeTypeLacros,
			Fixture:           "lacrosStartedByData",
			ExtraData:         []string{launcher.DataArtifact},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func SmartDim(ctx context.Context, s *testing.State) {
	const (
		dbusName            = "org.chromium.MlDecisionService"
		dbusPath            = dbus.ObjectPath("/org/chromium/MlDecisionService")
		dbusInterfaceMethod = "org.chromium.MlDecisionService.ShouldDeferScreenDim"

		eventHistogramName  = "MachineLearningService.SmartDimModel.ExecuteResult.Event"
		sourceHistogramName = "PowerML.SmartDimFeature.WebPageInfoSource"
		timeout             = 60 * time.Second
	)
	// TODO(crbug.com/1127165): Remove the artifactPath argument when we can use Data in fixtures.
	var artifactPath string
	if s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
		artifactPath = s.DataPath(launcher.DataArtifact)
	}
	cr, l, _, err := lacros.Setup(ctx, s.FixtValue(), artifactPath, s.Param().(lacros.ChromeType))
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacrosChrome(ctx, l)

	_, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
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

	waitForHistogram := func(name string, base *metrics.Histogram) *metrics.Histogram {
		s.Logf("Waiting for %v histogram", name)
		var histogram *metrics.Histogram
		var err error
		if base == nil {
			histogram, err = metrics.WaitForHistogram(ctx, tconn, name, timeout)
		} else {
			histogram, err = metrics.WaitForHistogramUpdate(ctx, tconn, name, base, timeout)
		}

		if err != nil {
			s.Fatalf("Failed to get histogram %q: %v", name, err)
		}
		s.Logf("Got histogram %q: %v", name, histogram)
		return histogram
	}

	call()
	eventHistogramBase := waitForHistogram(eventHistogramName, nil)
	sourceHistogramBase := waitForHistogram(sourceHistogramName, nil)

	call()
	eventHistogramUpdate := waitForHistogram(eventHistogramName, eventHistogramBase)
	sourceHistogramUpdate := waitForHistogram(sourceHistogramName, sourceHistogramBase)

	expectedEventBucket := metrics.HistogramBucket{Min: 0, Max: 1, Count: 1}
	if len(eventHistogramUpdate.Buckets) != 1 || eventHistogramUpdate.Buckets[0] != expectedEventBucket {
		s.Errorf("Unexpected event histogram update: want %+v, got %+v", expectedEventBucket, eventHistogramUpdate)
	}

	var expectedSourceBucket metrics.HistogramBucket
	if s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
		expectedSourceBucket = metrics.HistogramBucket{Min: 1, Max: 2, Count: 1}
	} else {
		expectedSourceBucket = metrics.HistogramBucket{Min: 0, Max: 1, Count: 1}
	}
	if len(sourceHistogramUpdate.Buckets) != 1 || sourceHistogramUpdate.Buckets[0] != expectedSourceBucket {
		s.Fatalf("Unexpected source histogram update: want %+v, got %+v", expectedSourceBucket, sourceHistogramUpdate)
	}
}
