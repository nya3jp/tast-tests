// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testCase struct {
	name              string
	systemLoadCommand string
	repetition        int
	quipperArgs       []string
	disableCPUIdle    bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PerfTool,
		Desc: "Tests D-Bus methods related to PerfTool",
		Contacts: []string{
			"shantuo@google.com",
			"cwp-team@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "generic",
			Val: []testCase{
				{
					name:              "idle_single",
					systemLoadCommand: "sleep 1",
					repetition:        1,
					quipperArgs: []string{"--duration", "4", "--",
						"record", "-a", "-e", "cycles", "-c", "1000003"},
				},
				{
					name:              "idle_repeated",
					systemLoadCommand: "sleep 1",
					repetition:        3,
					quipperArgs: []string{"--duration", "1", "--",
						"record", "-a", "-e", "cycles", "-c", "1000003"},
				},
				{
					name:              "busy_single",
					systemLoadCommand: "ls",
					repetition:        1,
					quipperArgs: []string{"--duration", "4", "--",
						"record", "-a", "-e", "cycles", "-c", "1000003"},
				},
				{
					name:              "busy_repeated",
					systemLoadCommand: "ls",
					repetition:        3,
					quipperArgs: []string{"--duration", "1", "--",
						"record", "-a", "-e", "cycles", "-c", "1000003"},
				},
			},
		}, {
			Name: "etm",
			Val: []testCase{
				{
					name:              "idle_single",
					systemLoadCommand: "sleep 1",
					repetition:        1,
					quipperArgs: []string{"--duration", "4", "--run_inject",
						"--inject_args", "inject;--itrace=i512il;--strip",
						"--", "record", "-e", "cs_etm/autofdo/", "-a", "-N"},
					disableCPUIdle: true,
				},
				{
					name:              "busy_single",
					systemLoadCommand: "ls",
					repetition:        1,
					quipperArgs: []string{"--duration", "4", "--run_inject",
						"--inject_args", "inject;--itrace=i512il;--strip",
						"--", "record", "-e", "cs_etm/autofdo/", "-a", "-N"},
					disableCPUIdle: true,
				},
			},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform("trogdor", "herobrine")),
		}},
	})
}

// PerfTool tests D-bus methods related to debugd's PerfTool.
func PerfTool(ctx context.Context, s *testing.State) {
	dbgd, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd D-Bus service: ", err)
	}

	if err := testGetPerfOutputV2(ctx, s, dbgd); err != nil {
		s.Error("Failed to verify GetPerfOutputV2: ", err)
	}
}

func testGetPerfOutputV2(ctx context.Context, s *testing.State, d *debugd.Debugd) error {
	for _, tc := range s.Param().([]testCase) {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			done := make(chan struct{})
			defer close(done)

			// Run the system load command to mimic the system load scenario.
			go func() {
				cmd := testexec.CommandContext(ctx, tc.systemLoadCommand)
				for {
					select {
					case <-done:
						return
					default:
						cmd.Run()
					}
				}
			}()

			for i := 0; i < tc.repetition; i++ {
				// For ETM collection tests only, we need to check cpuidle is properly
				// disabled during the collection and restored afterwards.
				if tc.disableCPUIdle {
					time.AfterFunc(time.Second, func() {
						if !checkCPUIdleDisabled(true, s) {
							s.Error("CPU Idle state not disabled during ETM collection")
						}
					})
				}

				perfData, perfStat, status, err := d.GetPerfOutputV2(ctx, tc.quipperArgs, tc.disableCPUIdle)
				if err != nil {
					s.Fatal("GetPerfOutputV2 failed: ", err)
				}
				if status != 0 {
					s.Fatalf("GetPerfOutputV2() returned status %d", status)
				}
				if tc.disableCPUIdle && !checkCPUIdleDisabled(false, s) {
					s.Error("CPU Idle state not restored after ETM collection")
				}
				if len(perfData) == 0 && len(perfStat) == 0 {
					s.Fatal("GetPerfOutputV2() returned no data")
				}
				if len(perfData) > 0 && len(perfStat) > 0 {
					s.Fatal("GetPerfOutputV2() returned both perf_data and perf_stat")
				}

				var result []byte
				var resultType string
				if len(perfData) > 0 {
					result = perfData
					resultType = "perfData"
				} else {
					result = perfStat
					resultType = "perfStat"
				}
				s.Logf("GetPerfOutputV2() returned %d bytes of type %s", len(result), resultType)
				if len(result) < 20 {
					s.Fatal("Perf output is too small")
				}
				// TODO(shantuo): remove
				if i == 0 {
					writeOutput(s, tc.name, result)
				}

				if bytes.HasPrefix(result, []byte("<process exited with status: ")) {
					s.Fatalf("Quipper failed: %s", string(result))
				}
			}

		})
	}
	return nil
}

// checkCPUIdleDisabled verifies whether all CPU's idle states match the given
// disabled status. It does not need to return any error since this is the same
// functionality implemented in the debugd perf_tool function, which will fail
// before this is called upon errors.
func checkCPUIdleDisabled(disabled bool, s *testing.State) bool {
	const cpuPossiblePath = "/sys/devices/system/cpu/possible"
	const cpuidlePathPat = "/sys/devices/system/cpu/cpu%d/cpuidle/state%d/disable"
	b, err := ioutil.ReadFile(cpuPossiblePath)
	if err != nil {
		return false
	}
	var min, max int
	if _, err := fmt.Sscanf(string(b), "%d-%d", &min, &max); err != nil {
		return false
	}
	for cpu := min; cpu <= max; cpu++ {
		for state := 0; ; state++ {
			cpuidlePath := fmt.Sprintf(cpuidlePathPat, cpu, state)
			f, err := os.Open(cpuidlePath)
			if err != nil {
				if os.IsNotExist(err) {
					break
				}
				return false
			}
			disable := make([]byte, 1)
			if n, err := f.Read(disable); err != nil || n != 1 {
				return false
			}
			if (disable[0] == '1') != disabled {
				return false
			}
		}
	}
	return true
}

func writeOutput(s *testing.State, filename string, content []byte) {
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), filename), content, 0644); err != nil {
		s.Errorf("Write %s failed: %v", filename, err)
	}
}
