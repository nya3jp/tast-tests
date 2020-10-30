// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"

	// "chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type testParams struct {
	name             string
	backlightPercent int
	sampleInterval   time.Duration
}

type powerBenchmarkParams struct {
	numIterations int
	minDuration   int
	maxDuration   int
	tests         []testParams
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerBenchmark,
		Desc: "Tests the resolution of power metrics for different task durations",
		Contacts: []string{
			"mblsha@google.com",
		},
		Attr: []string{
			"group:mainline",
		},
		Timeout:      999 * time.Hour,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{Name: "full", Val: powerBenchmarkParams{numIterations: 10, minDuration: 1, maxDuration: 30,
				// backlightLevels: []int{0, 50, 100, -1, -2}}},
				// -1: "idle, backlight: 0%, 100%, 100%, 0%"
				// -2: "idle, backlight: 0%, 50%, 100%, 0%"
				tests: []testParams{
					{name: "idle, backlight 0%", backlightPercent: 0, sampleInterval: 0 * time.Second},
					{name: "idle, backlight 0%; sample 1s", backlightPercent: 0, sampleInterval: time.Second / 1},
					{name: "idle, backlight 0%; sample 0.5s", backlightPercent: 0, sampleInterval: time.Second / 2},
					{name: "idle, backlight 0%; sample 0.1s", backlightPercent: 0, sampleInterval: time.Second / 10},
				}}},
			{Name: "sanity", Val: powerBenchmarkParams{numIterations: 10, minDuration: 1, maxDuration: 1,
				tests: []testParams{
					{name: "idle, backlight 0%", backlightPercent: 0, sampleInterval: 0 * time.Second},
					{name: "idle, backlight 0%; sample 0.1s", backlightPercent: 0, sampleInterval: time.Second / 10},
				}}},
			{Name: "test_backlight", Val: powerBenchmarkParams{numIterations: 10, minDuration: 1, maxDuration: 1,
				tests: []testParams{
					{name: "idle, backlight: 0%, 50%, 100%, 0%", backlightPercent: -2, sampleInterval: 0 * time.Second},
					{name: "idle, backlight: 0%, 50%, 100%, 0%; sample 0.1s", backlightPercent: -2, sampleInterval: time.Second / 10},
				}}},
		},
	})
}

// Samples momentary power from battery discharge to first calculate total power used in Wh, and then converts to W.
func samplePower(ctx context.Context, momentaryPowerW func() float64, sampleInterval time.Duration, quit chan struct{}, result chan float64) {
	if sampleInterval.Seconds() <= 0 {
		result <- 0
		return
	}

	startTime := time.Now()
	ticker := time.NewTicker(sampleInterval)

	lastTime := time.Now()
	totalPowerWh := 0.0

	updateTotalPower := func(t time.Time) {
		duration := t.Sub(lastTime)
		totalPowerWh += momentaryPowerW() * duration.Seconds() / 3600
		lastTime = t
	}

	for {
		select {
		case <-quit:
			updateTotalPower(time.Now())
			result <- totalPowerWh / time.Now().Sub(startTime).Seconds() * 3600
			return
		case t := <-ticker.C:
			updateTotalPower(t)
		}
	}
}

// power/read_utils.go
// readFirstLine reads the first line from a file.
// Line feed character will be removed to ease converting the string
// into other types.
func readFirstLine(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	// Reader.ReadString returns error iff line does not end in \n. We can
	// ignore this error.
	lineContent, _ := reader.ReadString('\n')
	return strings.TrimSuffix(lineContent, "\n"), nil
}

// readFloat64 reads a line from a file and converts it into float64.
func readFloat64(filePath string) (float64, error) {
	str, err := readFirstLine(filePath)
	if err != nil {
		return 0., err
	}
	return strconv.ParseFloat(str, 64)
}

// readInt64 reads a line from a file and converts it into int64.
func readInt64(filePath string) (int64, error) {
	str, err := readFirstLine(filePath)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(str, 10, 64)
}

func readTemperature(filePath string) (float64, error) {
	tempFile := path.Join(filePath, "temp")
	temp, err := readInt64(tempFile)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot read temperature from %s", tempFile)
	}
	return float64(temp) / 1000, nil
}

func readLoadAverage() (string, error) {
	return readFirstLine("/proc/loadavg")
}

func PowerBenchmark(ctx context.Context, s *testing.State) {
	p, ok := s.Param().(powerBenchmarkParams)
	if !ok {
		s.Fatal("Failed to convert test params to benchmarkParams")
	}

	// Ensure a consistent state with regards to Chrome: fresh session, no open windows.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	deadline, ok := ctx.Deadline()
	testing.ContextLog(ctx, deadline, ok)

	thermalSensors, err := power.ListSysfsThermalSensors(ctx)
	if err != nil {
		s.Fatalf("unable to get thermalSensors ", err)
	}
	// testing.ContextLog(ctx, thermalSensors)
	cpuThermal, ok := thermalSensors["x86_pkg_temp"]
	if !ok {
		cpuThermal, ok = thermalSensors["cpu_thermal"]
		if !ok {
			s.Fatalf("no x86_pkg_temp/cpu_thermal thermal sensor: ", thermalSensors)
		}
	}

	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Some configuration actions need a test connection to Chrome.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatalf("failed to connect to test API: ", err)
	}

	// setup.Setup configures a DUT for a test, and cleans up after.
	sup, cleanup := setup.New("mblsha power benchmark")
	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}()

	// Add the default power test configuration.
	sup.Add(setup.PowerTest(ctx, tconn, setup.ForceBatteryDischarge))
	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	batteryPaths, err := power.ListSysfsBatteryPaths(ctx)
	if err != nil {
		s.Fatal("Failed to get battery paths: ", err)
	}

	if len(batteryPaths) != 1 {
		s.Fatal("device has multiple batteries: ", batteryPaths)
	}

	//----------------------------------------------------------------------------

	// Some devices won't start reporting any discharge when a battery reads 100%.
	{
		battery_start, err := power.ReadBatteryEnergy(batteryPaths)
		if err != nil {
			s.Fatal("failed to read battery energy ", err)
		}

		for {
			battery_drain_duration_sec := 10
			newCtx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(battery_drain_duration_sec+1)*time.Second))
			defer cancel()

			// cmd := testexec.CommandContext(newCtx, "stress-ng", "--cpu", "1", "--timeout", strconv.Itoa(battery_drain_duration_sec)+"s")
			// if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			// 	s.Fatalf("failed to run stress-ng ", err)
			// }
			setup.SetBacklightPercent(ctx, 100)
			if err := testing.Sleep(ctx, time.Duration(battery_drain_duration_sec)*time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			current_battery, err := power.ReadBatteryEnergy(batteryPaths)
			if err != nil {
				s.Fatal("failed to read battery energy ", err)
			}

			if current_battery != battery_start {
				break
			}
			testing.ContextLog(newCtx, "Draining the battery to start reporting discharge ", current_battery)
		}
	}

	// testing.ContextLog(ctx, "10s Cooldown after CPU-intensive period to update battery power metrics")
	// if err := testing.Sleep(ctx, 10*time.Second); err != nil {
	// 	s.Fatal("Failed to sleep: ", err)
	// }

	type Result struct {
		timestamp    time.Time
		duration_sec int
		test_name    string

		// delta power measurements
		rapl_w    float64
		battery_w float64
		sample_w  float64

		// momentary power measurements
		momentary_battery_w_start float64
		momentary_battery_w_end   float64
		battery_percentage_start  float64
		battery_percentage_end    float64

		// load average
		la_start string
		la_end   string

		// x86_pkg_temp
		t_start float64
		t_end   float64
	}
	var results []Result

	readMomentaryPowerW := func() float64 {
		result, err := power.ReadSystemMomentaryPower(batteryPaths)
		if err != nil {
			s.Fatal("failed to read battery momentary power: ", err)
		}
		return result
	}

loop:
	for _, test := range p.tests {
		for test_duration := p.minDuration; test_duration <= p.maxDuration; test_duration += 1 {
			for i := 0; i < p.numIterations; i += 1 {
				newCtx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(1+test_duration)*time.Second))
				defer cancel()

				temperature_start, err := readTemperature(cpuThermal)
				if err != nil {
					s.Fatalf("failed to read CPU temperature ", err)
				}
				la_start, err := readLoadAverage()
				if err != nil {
					s.Fatalf("failed to read load average ", err)
				}

				battery_energy_start, err := power.ReadBatteryEnergy(batteryPaths)
				if err != nil {
					s.Fatal("failed to read battery energy ", err)
				}
				momentary_battery_w_start := readMomentaryPowerW()
				battery_percentage_start, err := power.ReadBatteryChargePercentage(batteryPaths)
				if err != nil {
					s.Fatal("failed to read battery percentage: ", err)
				}

				// uses intel RAPL (aka simulated power metrics, supported only on Intel platforms)
				snapshot, err := power.NewRAPLSnapshot()
				if err != nil {
					s.Fatalf("failed to create RAPL Snapshot", err)
				}
				if snapshot != nil {
					if _, err := snapshot.DiffWithCurrentRAPLAndReset(); err != nil {
						s.Fatalf("failed to collect initial RAPL metrics ", err)
					}
				}

				quitSampling := make(chan struct{}, 1)
				samplingResult := make(chan float64)
				go samplePower(ctx, readMomentaryPowerW, test.sampleInterval, quitSampling, samplingResult)

				// test_name := "stress-ng --cpu " + strconv.Itoa(stress_n_cpu),
				// human_duration := strconv.Itoa(test_duration) + "s"
				// cmd := testexec.CommandContext(newCtx, "stress-ng", "--cpu", strconv.Itoa(stress_n_cpu), "--timeout", human_duration)
				// if err := cmd.Run(testexec.DumpLogOnError); err != nil {
				// 	s.Fatalf("failed to run stress-ng ", err)
				// }

				test_name := test.name
				if test.backlightPercent == -1 {
					zero_backlight_duration := time.Duration(float64(test_duration)/4) * time.Second
					full_backlight_duration := time.Duration(float64(test_duration)/2) * time.Second

					setup.SetBacklightPercent(ctx, 0)
					if err := testing.Sleep(ctx, zero_backlight_duration); err != nil {
						s.Fatal("Failed to sleep: ", err)
					}

					setup.SetBacklightPercent(ctx, 100)
					if err := testing.Sleep(ctx, full_backlight_duration); err != nil {
						s.Fatal("Failed to sleep: ", err)
					}

					setup.SetBacklightPercent(ctx, 0)
					if err := testing.Sleep(ctx, zero_backlight_duration); err != nil {
						s.Fatal("Failed to sleep: ", err)
					}
				} else if test.backlightPercent == -2 {
					quarter_duration := time.Duration(float64(test_duration)/4) * time.Second

					setup.SetBacklightPercent(ctx, 0)
					if err := testing.Sleep(ctx, quarter_duration); err != nil {
						s.Fatal("Failed to sleep: ", err)
					}

					setup.SetBacklightPercent(ctx, 50)
					if err := testing.Sleep(ctx, quarter_duration); err != nil {
						s.Fatal("Failed to sleep: ", err)
					}

					setup.SetBacklightPercent(ctx, 100)
					if err := testing.Sleep(ctx, quarter_duration); err != nil {
						s.Fatal("Failed to sleep: ", err)
					}

					setup.SetBacklightPercent(ctx, 0)
					if err := testing.Sleep(ctx, quarter_duration); err != nil {
						s.Fatal("Failed to sleep: ", err)
					}
				} else {
					setup.SetBacklightPercent(ctx, test.backlightPercent)
					if err := testing.Sleep(ctx, time.Duration(test_duration)*time.Second); err != nil {
						s.Fatal("Failed to sleep: ", err)
					}
				}

				quitSampling <- struct{}{}
				sample_watts := <-samplingResult

				testing.ContextLog(newCtx, test_name, ": ", test_duration, "s, run #", i+1, " of ", p.numIterations)

				rapl_watts := 0.0
				if snapshot != nil {
					energy, err := snapshot.DiffWithCurrentRAPLAndReset()
					if err != nil {
						s.Fatalf("failed to collect initial RAPL metrics", err)
					}
					rapl_watts = energy.Total() / float64(test_duration)
				}

				battery_energy_end, err := power.ReadBatteryEnergy(batteryPaths)
				if err != nil {
					s.Fatal("failed to read battery energy ", err)
				}
				momentary_battery_w_end := readMomentaryPowerW()
				battery_percentage_end, err := power.ReadBatteryChargePercentage(batteryPaths)
				if err != nil {
					s.Fatal("failed to read battery percentage: ", err)
				}

				if battery_energy_start < battery_energy_end {
					// mblsha: Happened to me on a poppy/ekko.
					s.Errorf("battery_energy_start (", battery_energy_start, ") < battery_energy_end (", battery_energy_end, "): is charger not disabled?")
					// don't want negative numbers poisoning the average.
					battery_energy_end = battery_energy_start
				}
				// convert Wh to W
				battery_watts := (battery_energy_start - battery_energy_end) / float64(test_duration) * 3600

				temperature_end, err := readTemperature(cpuThermal)
				if err != nil {
					s.Fatalf("failed to read CPU temperature ", err)
				}
				la_end, err := readLoadAverage()
				if err != nil {
					s.Fatalf("failed to read load average ", err)
				}

				r := Result{
					timestamp:    time.Now(),
					duration_sec: test_duration,
					test_name:    test_name,

					rapl_w:    rapl_watts,
					battery_w: battery_watts,
					sample_w:  sample_watts,

					momentary_battery_w_start: momentary_battery_w_start,
					momentary_battery_w_end:   momentary_battery_w_end,
					battery_percentage_start:  battery_percentage_start,
					battery_percentage_end:    battery_percentage_end,

					la_start: la_start,
					la_end:   la_end,

					t_start: temperature_start,
					t_end:   temperature_end}

				testing.ContextLog(newCtx, "battery_energy_start: ", battery_energy_start, "; battery_energy_end: ", battery_energy_end)
				testing.ContextLog(newCtx, r)
				results = append(results, r)

				// We want at least some of the results in case the DUT dies during the test.
				if battery_energy_end < 2 {
					break loop
				}
			}
		}
	}

	testing.ContextLog(ctx, "\n\n\n")
	csv := "timestamp,duration_sec,test_name,rapl_w,battery_w,sample_w,momentary_battery_w_start,momentary_battery_w_end,battery_percentage_start,battery_percentage_end,t_start,t_end,la_start,la_end\n"
	for _, r := range results {
		csv += fmt.Sprintf("\"%s\"", r.timestamp.Format(time.RFC3339))
		csv += fmt.Sprintf(",%d", r.duration_sec)
		csv += fmt.Sprintf(",\"%s\"", r.test_name)
		csv += fmt.Sprintf(",%f", r.rapl_w)
		csv += fmt.Sprintf(",%f", r.battery_w)
		csv += fmt.Sprintf(",%f", r.sample_w)
		csv += fmt.Sprintf(",%f", r.momentary_battery_w_start)
		csv += fmt.Sprintf(",%f", r.momentary_battery_w_end)
		csv += fmt.Sprintf(",%f", r.battery_percentage_start)
		csv += fmt.Sprintf(",%f", r.battery_percentage_end)
		csv += fmt.Sprintf(",%f", r.t_start)
		csv += fmt.Sprintf(",%f", r.t_end)
		csv += fmt.Sprintf(",\"%s\"", r.la_start)
		csv += fmt.Sprintf(",\"%s\"", r.la_end)
		csv += "\n"
	}
	testing.ContextLog(ctx, "results:\n\n", csv, "\n\n\n")

	csvFile, err := os.OpenFile(filepath.Join(s.OutDir(), "power_benchmark_results.csv"),
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		0644)
	if err != nil {
		s.Fatalf("failed to open file for csv output: ", err)
	}

	_, err = csvFile.WriteString(csv)
	if err != nil {
		s.Fatalf("failed to write csv: ", err)
	}
}
