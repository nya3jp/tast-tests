// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

type smartDimParam struct {
	browserType        browser.Type
	useFlatbufferModel bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SmartDim,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Check the SmartDim can make decision with ML Service",
		Contacts:     []string{"alanlxl@chromium.org"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "ml_service", "smartdim"},
		Params: []testing.Param{{
			Val: &smartDimParam{
				browserType:        browser.TypeAsh,
				useFlatbufferModel: false,
			},
			Fixture: "chromeFastHistogramsAndBuiltinSmartDimModel",
		}, {
			Name: "flatbuffer",
			Val: &smartDimParam{
				browserType:        browser.TypeAsh,
				useFlatbufferModel: true,
			},
			Fixture: "chromeFastHistograms",
		}, {
			Name: "lacros",
			Val: &smartDimParam{
				browserType:        browser.TypeLacros,
				useFlatbufferModel: false,
			},
			Fixture:           "lacrosFastHistogramsAndBuiltinSmartDimModel",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros_stable"},
		}},
	})
}

func SmartDim(ctx context.Context, s *testing.State) {
	const (
		dbusName            = "org.chromium.MlDecisionService"
		dbusPath            = dbus.ObjectPath("/org/chromium/MlDecisionService")
		dbusInterfaceMethod = "org.chromium.MlDecisionService.ShouldDeferScreenDim"
		timeout             = 60 * time.Second
	)

	histogramNames := []string{
		"MachineLearningService.SmartDimModel.ExecuteResult.Event",
		"PowerML.SmartDimComponent.WorkerType",
		"PowerML.SmartDimFeature.WebPageInfoSource"}

	param := s.Param().(*smartDimParam)

	cr, l, _, err := lacros.Setup(ctx, s.FixtValue(), param.browserType)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(ctx, l)

	_, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}

	call := func(ctx context.Context) error {
		s.Log("Asking /org/chromium/MlDecisionService for Smart Dim decision")
		var state bool
		if err := obj.CallWithContext(ctx, dbusInterfaceMethod, 0).Store(&state); err != nil {
			return errors.Wrap(err, "failed to get Smart Dim decision")
		}
		s.Log("Smart Dim decision is ", state)
		return nil
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if param.useFlatbufferModel {
		s.Log("Trigger component update and check the downloadable model works")

		if err = tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.loadSmartDimComponent)`); err != nil {
			s.Fatal("Running autotestPrivate.loadSmartDimComponent failed: ", err)
		}
	}

	// Wait until all pending updates for these three histograms are completed.
	// This is because event histogram is CrOS metrics and Chromium has a delayed collection cycle for them.
	// Its pending updates may be merged to diffs after the dbus call, leading to unexpected result.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		recorder, err := metrics.StartRecorder(ctx, tconn, histogramNames...)
		if err != nil {
			return errors.Wrap(err, "failed to start recorder")
		}
		diffs, err := recorder.WaitAny(ctx, tconn, 3*time.Second)
		if diffs != nil {
			s.Log("Got pending updates, try again")
			return errors.New("got pending updates")
		}
		return nil
	}, &testing.PollOptions{Timeout: 1 * time.Minute, Interval: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for completion of all pending updates : ", err)
	}

	histograms, err := metrics.RunAndWaitAll(ctx, tconn, 3*time.Second, call, histogramNames...)
	if err != nil {
		s.Fatal("Failed to run and wait all histograms")
	}

	// SmartDimModel.ExecuteResult.Event=0 means model is invoked successfully.
	expectedEventBucket := metrics.HistogramBucket{Min: 0, Max: 1, Count: 1}
	if len(histograms[0].Buckets) != 1 || histograms[0].Buckets[0] != expectedEventBucket {
		s.Errorf("Unexpected event histogram update: want %+v, got %+v", expectedEventBucket, histograms[0])
	}

	// WorkerType 0 means builtin model, 1 means flatbuffer model.
	var expectedWorkerBucket metrics.HistogramBucket
	if param.useFlatbufferModel {
		expectedWorkerBucket = metrics.HistogramBucket{Min: 1, Max: 2, Count: 1}
	} else {
		expectedWorkerBucket = metrics.HistogramBucket{Min: 0, Max: 1, Count: 1}
	}

	if len(histograms[1].Buckets) != 1 || histograms[1].Buckets[0] != expectedWorkerBucket {
		s.Errorf("Unexpected worker histogram update: want %+v, got %+v", expectedWorkerBucket, histograms[1])
	}

	// WebPageInfoSource 0 means Ash , 1 means Lacros.
	var expectedSourceBucket metrics.HistogramBucket
	if param.browserType == browser.TypeLacros {
		expectedSourceBucket = metrics.HistogramBucket{Min: 1, Max: 2, Count: 1}
	} else {
		expectedSourceBucket = metrics.HistogramBucket{Min: 0, Max: 1, Count: 1}
	}
	if len(histograms[2].Buckets) != 1 || histograms[2].Buckets[0] != expectedSourceBucket {
		s.Fatalf("Unexpected source histogram update: want %+v, got %+v", expectedSourceBucket, histograms[2])
	}
}
