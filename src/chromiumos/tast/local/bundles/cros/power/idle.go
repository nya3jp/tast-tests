// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	cupstart "chromiumos/tast/common/upstart"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Idle,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Collects data on idle with Chrome logged in",
		Contacts:     []string{"hidehiko@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      13 * time.Minute,
		Params: []testing.Param{{
			Name: "ash",
		}},
	})
}

func stopPowerDrawServices(ctx context.Context) (func(ctx context.Context), error) {
	targets := []string{"fwupd", "powerd", "update-engine", "vnc"}
	var stopped []string
	cleanup := func(ctx context.Context) {
		// Ensure the services in the reverse order of stopped.
		for i := 0; i < len(stopped)/2; i++ {
			stopped[i], stopped[len(stopped)-i-1] = stopped[len(stopped)-i-1], stopped[i]
		}
		for _, job := range stopped {
			err := upstart.EnsureJobRunning(ctx, job)
			if err != nil {
				testing.ContextLogf(ctx, "Failed to restore the job %s: %v", job, err)
			}
		}
	}

	for _, job := range targets {
		goal, _, _, err := upstart.JobStatus(ctx, job)
		if err != nil {
			continue
		}
		if goal != cupstart.StopGoal {
			if err := upstart.StopJob(ctx, job); err != nil {
				cleanup(ctx)
				return nil, err
			}
		}
		stopped = append(stopped, job)
	}
	return cleanup, nil
}

// TODO(hidehiko): Consolidate with perf.BatteryInfoTracker.
type batteryState struct {
	sysfsPowerPath string
	metrics        map[string]perf.Metric
}

var _ perf.TimelineDatasource = &batteryState{}

func (b *batteryState) Setup(_ context.Context, prefix string) error {
	// Obtain the status before modifing internal state.
	status, err := power.ReadBatteryStatus(b.sysfsPowerPath)
	if err != nil {
		return err
	}

	b.metrics = map[string]perf.Metric{}
	b.metrics["percent"] = perf.Metric{
		Name:      prefix + "percent",
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}
	if status == power.BatteryStatusDischarging {
		b.metrics["energy rate (W)"] = perf.Metric{
			Name:      prefix + "energyrate",
			Unit:      "W",
			Direction: perf.BiggerIsBetter,
			Multiple:  true,
		}
	}
	return nil
}

func (b *batteryState) Start(_ context.Context) error {
	return nil
}

var fieldRe = regexp.MustCompile(`  (\w+):\s+(\d+)`)

func (b *batteryState) Snapshot(ctx context.Context, pv *perf.Values) error {
	out, err := testexec.CommandContext(ctx, "power_supply_info").Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}
	batteryFound := false
	for _, line := range strings.Split(string(out), "\n") {
		if !batteryFound {
			batteryFound = line == "Device: Battery"
			continue
		}
		m := fieldRe.FindStringSubmatch(line)
		if m != nil {
			metric, ok := b.metrics[m[1]]
			if !ok {
				continue
			}
			v, err := strconv.ParseFloat(m[2], 64)
			if err != nil {
				return err
			}
			pv.Append(metric, v)
		}
	}

	return nil
}

func (b *batteryState) Stop(_ context.Context, _ *perf.Values) error {
	return nil
}

func newIdleTimeline(ctx context.Context) (*perf.Timeline, error) {
	var srcs []perf.TimelineDatasource

	// Add Power related loggers.
	if sysfsPowerPath, err := power.SysfsBatteryPath(ctx); err != nil {
		if err != power.ErrNoBattery {
			return nil, err
		}
	} else {
		srcs = append(srcs, &batteryState{sysfsPowerPath: sysfsPowerPath})
	}
	srcs = append(srcs, power.NewRAPLPowerMetrics())

	// Add CPUIdle logger.
	srcs = append(srcs, power.NewCpuidleStateMetrics())
	return perf.NewTimeline(ctx, srcs)
}

func Idle(ctx context.Context, s *testing.State) {
	// Set up.

	// force discharge. Do nothing.

	su, cleanup := setup.New("power.idle")
	defer cleanup(ctx)
	su.Add(setup.SetBacklightLux(ctx, 150))
	// TODO: SetBatteryDischarge reports an error on some devices.
	// su.Add(setup.SetBatteryDischarge(ctx, 2.0))
	if err := su.Check(ctx); err != nil {
		s.Fatal("Test set up failed: ", err)
	}

	bts, err := bluetooth.Adapters(ctx)
	if err != nil {
		s.Fatal("Bluetooth adapters fail to be created: ", err)
	}

	// TODO(hidehiko): support ARC variations.
	cr, err := chrome.New(ctx,
		// --disable-sync disables test account info sync, eg. Wi-Fi credentials,
		// so that each test run does not remember info from last test run.
		chrome.ExtraArgs("--disable-sync"),
		// b/228256145 to avoid powerd restart.
		chrome.DisableFeatures("FirmwareUpdaterApp"))
	if err != nil {
		s.Fatal("Failed to login session: ", err)
	}
	defer cr.Close(ctx)

	br := cr.Browser()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get ash tconn: ", err)
	}
	conn, err := br.NewConn(ctx, "about:blank")
	if err != nil {
		s.Fatal("Failed to open a blank new tab: ", err)
	}
	defer conn.Close()

	w, err := ash.WaitForAnyWindow(ctx, tconn, ash.BrowserTypeMatch(browser.TypeAsh))
	if err != nil {
		s.Fatal("Failed to open a browser window: ", err)
	}
	if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized); err != nil {
		s.Fatal("Failed to maximize the browser window: ", err)
	}

	restore, err := stopPowerDrawServices(ctx)
	if err != nil {
		s.Fatal("Failed to stop power consuming services: ", err)
	}
	defer restore(ctx)

	timeline, err := newIdleTimeline(ctx)
	if err != nil {
		s.Fatal("Failed to create a timeline: ", err)
	}

	type Period struct {
		begin time.Time
		end   time.Time
	}
	result := map[string][]Period{}
	checkpoint := func(title string, p Period) {
		result[title] = append(result[title], p)
	}

	firstTime := true
	measure := func(title string) {
		// Warmup.
		duration := 20 * time.Second
		if firstTime {
			firstTime = false
			duration += 60 * time.Second
		}
		begin := time.Now()
		if err := testing.Sleep(ctx, duration); err != nil {
			s.Fatal("Failed to wait for warm up: ", err)
		}
		end := time.Now()
		checkpoint("warmup", Period{begin, end})

		// Actual measure with idle.
		begin = time.Now()
		if err := testing.Sleep(ctx, 120*time.Second); err != nil {
			s.Fatal("Failed to wait idling: ", err)
		}
		end = time.Now()
		checkpoint(title, Period{begin, end})
	}

	// Test1: display off, BT off.
	if err := power.SetDisplayPower(ctx, power.DisplayPowerAllOff); err != nil {
		s.Fatal("Failed to turn off display: ", err)
	}
	if err := timeline.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	measure("display-off_bluetooth-off")
	// measure_it(warmup_secs, idle_secs, 'display-off_bluetooth-off')

	// Test2: display default, BT off
	if err := power.SetDisplayPower(ctx, power.DisplayPowerAllOn); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}
	// measure_it(warmup_secs, idle_secs, 'display-off_bluetooth-off')
	measure("display-default_-bluetooth-off")

	// Test3: display default, BT on
	for _, bt := range bts {
		if err := bt.SetPowered(ctx, true); err != nil {
			s.Fatalf("Failed to set powered to bluetooth %s: %v", bt.Path(), err)
		}
	}
	// measure_it(warmup_secs, idle_secs, 'display-off_bluetooth-off')
	measure("display-default_bluetooth-on")

	// Test4: display off, BT on
	if err := power.SetDisplayPower(ctx, power.DisplayPowerAllOff); err != nil {
		s.Fatal("Failed to turn off display: ", err)
	}
	measure("display-off_bluetooth-on")

	pv, err := timeline.StopRecording(ctx)
	if err != nil {
		s.Fatal("Failed to stop recording: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to store the metrics: ", err)
	}

	// TODO(hidehiko): Create a json data so that dashboard can consume it.
}
