// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SensorPerf,
		Desc:         "Test ARC sensor system performance",
		Contacts:     []string{"arc-performance@google.com", "wvk@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Fixture: "arcBooted",
		Timeout: 2 * time.Minute,
	})
}

// latencyResult represents the average latency for a single sensor device in
// Android.
type latencyResult struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	NumEvents    int     `json:"numEvents"`
	AvgLatencyNs float64 `json:"avgLatencyNs"`
	AvgDelayNs   float64 `json:"avgDelayNs"`
}

func SensorPerf(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}
	a := s.FixtValue().(*arc.PreData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Could not initialize UI Automator: ", err)
	}
	defer d.Close(ctx)

	const (
		apkName      = "ArcSensorLatencyTest.apk"
		appName      = "org.chromium.arc.testapp.sensorlatency"
		activityName = ".MainActivity"
	)
	s.Log("Installing " + apkName)
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	s.Logf("Launching %s/%s", appName, activityName)
	act, err := arc.NewActivity(a, appName, activityName)
	if err != nil {
		s.Fatalf("Unable to create new activity %s/%s: %v", appName, activityName, err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Unable to launch %s/%s: %v", appName, activityName, err)
	}
	defer act.Stop(ctx, tconn)

	s.Log("Recording sensor events")
	startButton := d.Object(ui.ID("org.chromium.arc.testapp.sensorlatency:id/start_button"))
	if err := startButton.Click(ctx); err != nil {
		s.Fatal("Unable to click start button: ", err)
	}

	// Poll until the event count >= minEvents.
	const minEvents = 10000
	countView := d.Object(ui.ID("org.chromium.arc.testapp.sensorlatency:id/count"))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		txt, err := countView.GetText(ctx)
		if err != nil {
			return err
		}
		num, err := strconv.ParseInt(txt, 10, 64)
		if err != nil {
			return err
		}
		if num < minEvents {
			return errors.Errorf("not enough events; got %d, want >%d", num, minEvents)
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		s.Fatal("Failed to wait for events: ", err)
	}

	s.Log("Stopping recording")
	stopButton := d.Object(ui.ID("org.chromium.arc.testapp.sensorlatency:id/stop_button"))
	if err := stopButton.Click(ctx); err != nil {
		s.Fatal("Unable to click stop button: ", err)
	}

	// Poll until results view is non-empty.
	resultsView := d.Object(ui.ID("org.chromium.arc.testapp.sensorlatency:id/results"))
	var resultTxt string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		txt, err := resultsView.GetText(ctx)
		if err != nil {
			return err
		}
		if len(txt) == 0 {
			return errors.New("results view is empty")
		}
		resultTxt = txt
		return nil
	}, nil); err != nil {
		s.Fatal("Failed to wait for results: ", err)
	}

	var results []latencyResult
	if err := json.Unmarshal([]byte(resultTxt), &results); err != nil {
		s.Logf("Unable to unmarshal text: %q", resultTxt)
		s.Fatal("Failed to unmarshal latency results: ", err)
	}

	pv := perf.NewValues()
	for _, result := range results {
		s.Logf("%s(%s): n %d, latency %fms", result.Name, result.Type, result.NumEvents, result.AvgLatencyNs/float64(time.Millisecond))
		metricName := strings.Replace(result.Name, " ", "", -1)
		pv.Set(perf.Metric{
			Name:      metricName,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, result.AvgLatencyNs/float64(time.Millisecond))
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
