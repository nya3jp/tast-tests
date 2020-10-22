// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testArgsForPowerIdlePerf struct {
	setupOption setup.BatteryDischargeMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerIdlePerf,
		Desc: "Measures the battery drain of an idle system",
		Contacts: []string{
			"cwd@chromium.org",
			"arcvm-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Params: []testing.Param{{
			Name:              "noarc",
			ExtraSoftwareDeps: []string{"arc"}, // to prevent this from running on non-ARC boards
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.ForceBatteryDischarge,
			},
			Pre: chrome.LoggedIn(),
		}, {
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.ForceBatteryDischarge,
			},
			Pre: arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.ForceBatteryDischarge,
			},
			Pre: arc.Booted(),
		}, {
			Name:              "noarc_nobatterymetrics",
			ExtraSoftwareDeps: []string{"arc"}, // to prevent this from running on non-ARC boards
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.NoBatteryDischarge,
			},
			Pre: chrome.LoggedIn(),
		}, {
			Name:              "nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.NoBatteryDischarge,
			},
			Pre: arc.Booted(),
		}, {
			Name:              "vm_nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.NoBatteryDischarge,
			},
			Pre: arc.Booted(),
		}},
		Timeout: 15 * time.Minute,
	})
}

func PowerIdlePerf(ctx context.Context, s *testing.State) {
	const (
		iterationCount    = 30
		iterationDuration = 10 * time.Second
	)
	args := s.Param().(testArgsForPowerIdlePerf)

	// Give cleanup actions a minute to run, even if we fail by exceeding our
	// deadline.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		cr = s.PreValue().(arc.PreData).Chrome
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	sup, cleanup := setup.New("power idle perf")
	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}()

	sup.Add(setup.PowerTest(ctx, tconn, args.setupOption))
	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	metrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Interval(iterationDuration))
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}
	s.Log("Finished setup")

	// Wait until CPU is cooled down.
	cooldownTime, err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI)
	if err != nil {
		s.Fatal("CPU failed to cool down: ", err)
	}

	if err := metrics.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	if err := metrics.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	if err := testing.Sleep(ctx, iterationCount*iterationDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	p, err := metrics.StopRecording()
	if err != nil {
		s.Fatal("Error while recording power metrics: ", err)
	}

	cooldownTimeMetric := perf.Metric{Name: "cooldown_time", Unit: "ms", Direction: perf.SmallerIsBetter}
	p.Set(cooldownTimeMetric, float64(cooldownTime.Milliseconds()))

	// Report memory usage.
	arcPre, ok := s.PreValue().(arc.PreData)
	metricRegexp := regexp.MustCompile("[^a-zA-Z0-9]")
	if ok {
		memOut, err := arcPre.ARC.Command(ctx, "cat", "/proc/meminfo").Output()
		var valMemFree int64
		var valMemTotal int64
		if err != nil {
			s.Fatal("Failed to read from /proc/meminfo: ", err)
		} else {
			scanner := bufio.NewScanner(strings.NewReader(string(memOut)))
			for scanner.Scan() {
				// Line format example: MemTotal:      14863104 kB
				line := scanner.Text()
				tokens := strings.Fields(line)
				memName := tokens[0][:len(tokens[0])-1]
				// No special characters allowed in metric names.
				metricName := metricRegexp.ReplaceAllString(memName, "_")
				metricValue := tokens[1]
				metricUnit := tokens[2]

				if metricUnit != "kB" {
					s.Fatalf("Invalid metric unit. Found %s, expected kB", metricUnit)
				}

				valueInt, err := strconv.ParseInt(metricValue, 10, 64)
				if err != nil {
					s.Fatal("Invalid metric value: ", err)
				}

				if memName == "MemFree" {
					valMemFree = valueInt
				} else if memName == "MemTotal" {
					valMemTotal = valueInt
				}

				memMetric := perf.Metric{Name: "arc_mem_" + metricName, Unit: "kB", Direction: perf.SmallerIsBetter}
				p.Set(memMetric, float64(valueInt))
			}

			// TotalUsage = MemTotal - MemFree
			memMetric := perf.Metric{Name: "arc_mem_TotalUsage", Unit: "kB", Direction: perf.SmallerIsBetter}
			valMemUsage := valMemTotal - valMemFree
			if valMemUsage <= 0 {
				s.Fatalf("Invalid total memory usage: total = %d, free = %d", valMemTotal, valMemFree)
			}
			p.Set(memMetric, float64(valMemUsage))
		}
	} else {
		testing.ContextLog(ctx, "ARC not running, skipping memory metrics")
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
