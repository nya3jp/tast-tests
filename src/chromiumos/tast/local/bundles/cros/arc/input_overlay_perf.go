// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/gio"
	"chromiumos/tast/local/bundles/cros/arc/inputlatency"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	numEvents = 50
	waitMS    = 50
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputOverlayPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the performance of inputs for input overlay",
		Contacts:     []string{"pjlee@google.com", "cuicuiruan@google.com", "arc-app-dev@google.com", "arc-performance@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBootedWithInputOverlay",
		Data:         inputlatency.AndroidData(),
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
			}},
		Timeout: 5 * time.Minute,
	})
}

func InputOverlayPerf(ctx context.Context, s *testing.State) {
	gio.SetupTestApp(ctx, s, func(params gio.TestParams) error {
		// Start up keyboard.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to open keyboard")
		}
		defer kb.Close()
		// Start up UIAutomator.
		ui := uiauto.New(params.TestConn).WithTimeout(time.Minute)
		// Install the ARC host clock.
		if err := inputlatency.InstallArcHostClockClient(ctx, params.Arc, s); err != nil {
			return errors.Wrap(err, "could not install arc-host-clock-client")
		}

		// Click to close educational dialogue.
		if err := ui.LeftClick(nodewith.Name("Got it").HasClass("LabelButtonLabel"))(ctx); err != nil {
			return errors.Wrap(err, "failed to click educational dialog")
		}

		// Inject the described number of tap events.
		tapEventTimes := make([]int64, 0, numEvents)
		for i := 0; i < numEvents; i += 2 {
			if err := inputlatency.WaitForNextEventTime(ctx, params.Arc, &tapEventTimes, waitMS); err != nil {
				return errors.Wrap(err, "failed to generate event time")
			}
			if err := kb.AccelPress(ctx, "m"); err != nil {
				return errors.Wrap(err, "unable to inject key events")
			}

			if err := inputlatency.WaitForNextEventTime(ctx, params.Arc, &tapEventTimes, waitMS); err != nil {
				return errors.Wrap(err, "failed to generate event time")
			}
			if err := kb.AccelRelease(ctx, "m"); err != nil {
				return errors.Wrap(err, "unable to inject key events")
			}
		}

		// Calculate input latency and save metrics.
		tapPv := perf.NewValues()
		if err := evaluateLatency(ctx, params, tapEventTimes, "avgInputOverlayTapLatency", tapPv); err != nil {
			return errors.Wrap(err, "failed to evaluate")
		}
		if err := tapPv.Save(s.OutDir()); err != nil {
			return errors.Wrap(err, "failed saving perf data")
		}

		return nil
	})
}

// evaluateLatency gets event data, calculates the latency, and adds the result to performance metrics.
func evaluateLatency(ctx context.Context, params gio.TestParams, eventTimes []int64, perfName string, pv *perf.Values) error {
	// Get event received RTC times.
	events, err := gio.PopulateReceivedTimes(ctx, params, numEvents)
	if err != nil {
		return errors.Wrap(err, "could not receive event")
	}

	// Assign event RTC time.
	for i := range events {
		events[i].EventTimeNS = eventTimes[i]
	}

	mean, median, stdDev, max, min := inputlatency.CalculateMetrics(events, func(i int) float64 {
		return float64(events[i].RecvTimeNS-events[i].EventTimeNS) / 1000000.
	})
	testing.ContextLogf(ctx, "Latency (ms): mean %f median %f std %f max %f min %f", mean, median, stdDev, max, min)

	pv.Set(perf.Metric{
		Name:      perfName,
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, mean)
	return nil
}
