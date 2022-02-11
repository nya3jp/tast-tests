// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"

	"chromiumos/tast/local/debugd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testCase struct {
	durationSecs   int
	quipperArgs    []string
	disableCPUIdle bool
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
			Name: "cycles",
			Val: testCase{
				durationSecs: 4,
				quipperArgs: []string{"--duration", "", "--",
					"record", "-a", "-e", "cycles", "-c", "1000003"},
			},
		}, {
			Name: "etm",
			Val: testCase{
				durationSecs: 4,
				quipperArgs: []string{"--duration", "", "--run_inject",
					"--inject_args", "inject;--itrace=i512il;--strip",
					"--", "record", "-e", "cs_etm/autofdo/", "-a", "-N"},
				disableCPUIdle: true,
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

	testSingleCall(ctx, s, dbgd)
	testConsecutiveCalls(ctx, s, dbgd)
	testConcurrentCalls(ctx, s, dbgd)
	testStopEarly(ctx, s, dbgd)
}

func getPerfOutput(ctx context.Context, s *testing.State, d *debugd.Debugd, tc testCase) (*os.File, uint64, error) {
	for i, arg := range tc.quipperArgs {
		if arg == "--duration" && tc.quipperArgs[i+1] == "" {
			tc.quipperArgs[i+1] = strconv.Itoa(tc.durationSecs)
		}
	}

	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		s.Fatal("Failed to create status pipe: ", err)
	}
	defer wPipe.Close()

	sessionID, err := d.GetPerfOutputV2(ctx, tc.quipperArgs, tc.disableCPUIdle, wPipe)
	if err != nil {
		rPipe.Close()
		return nil, 0, err
	}
	if sessionID == 0 {
		s.Fatal("Invalid session ID from GetPerfOutputFd")
	}
	return rPipe, sessionID, nil
}

func checkPerfData(s *testing.State, result []byte) {
	s.Logf("GetPerfOutputV2() returned %d bytes of perf data", len(result))
	if len(result) < 20 {
		s.Fatal("Perf output is too small")
	}
	if bytes.HasPrefix(result, []byte("<process exited with status: ")) {
		s.Fatalf("Quipper failed: %s", string(result))
	}
}

func testSingleCall(ctx context.Context, s *testing.State, d *debugd.Debugd) {
	s.Run(ctx, "testSingleCall", func(ctx context.Context, s *testing.State) {
		tc := s.Param().(testCase)
		if tc.disableCPUIdle {
			time.AfterFunc(time.Second, func() {
				if !checkCPUIdleDisabled(true, s) {
					s.Error("CPU Idle state not disabled during ETM collection")
				}
			})
		}

		output, sessionID, err := getPerfOutput(ctx, s, d, tc)
		if err != nil {
			s.Fatal("Failed to call GetPerfOutputV2: ", err)
		}
		defer output.Close()

		s.Log("Session ID: ", sessionID)
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, output); err != nil {
			s.Fatal("Failed to read perf output: ", err)
		}
		// Wait 1 second to avoid race.
		testing.Sleep(ctx, time.Second)
		if tc.disableCPUIdle && !checkCPUIdleDisabled(false, s) {
			s.Error("CPU Idle state not restored after perf collection")
		}
		checkPerfData(s, buf.Bytes())
	})
}

func testConsecutiveCalls(ctx context.Context, s *testing.State, d *debugd.Debugd) {
	s.Run(ctx, "testConsecutiveCalls", func(ctx context.Context, s *testing.State) {
		tc := s.Param().(testCase)
		tc.durationSecs = 1
		for i := 0; i < 3; i++ {
			output, sessionID, err := getPerfOutput(ctx, s, d, tc)
			if err != nil {
				s.Fatal("Failed to call GetPerfOutputV2: ", err)
			}
			defer output.Close()

			s.Log("Session ID: ", sessionID)
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, output); err != nil {
				s.Fatal("Failed to read perf output: ", err)
			}
			checkPerfData(s, buf.Bytes())
		}
	})
}

func testConcurrentCalls(ctx context.Context, s *testing.State, d *debugd.Debugd) {
	s.Run(ctx, "testConcurrentCalls", func(ctx context.Context, s *testing.State) {
		tc := s.Param().(testCase)
		repetition := 3
		errc := make(chan error, repetition)
		var wg sync.WaitGroup
		for i := 0; i < repetition; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				output, sessionID, err := getPerfOutput(ctx, s, d, tc)
				if err != nil {
					errc <- err
					return
				}
				defer output.Close()
				s.Log("Session ID: ", sessionID)
				var buf bytes.Buffer
				if _, err := io.Copy(&buf, output); err != nil {
					s.Fatal("Failed to read perf output: ", err)
				}
				checkPerfData(s, buf.Bytes())
			}()
		}
		wg.Wait()
		close(errc)

		ec := 0
		for err := range errc {
			s.Log("\"Existing perf tool running\" error expected, got: ", err)
			ec++
		}
		if ec != repetition-1 {
			s.Errorf("Calling GetPerfOutputV2 %d times concurrently, got %d errors, want %d",
				repetition, ec, repetition-1)
		}
	})
}

func testStopEarly(ctx context.Context, s *testing.State, d *debugd.Debugd) {
	s.Run(ctx, "testStopEarly", func(ctx context.Context, s *testing.State) {
		tc := s.Param().(testCase)
		tc.durationSecs = 15
		stop := 4
		start := time.Now()

		if tc.disableCPUIdle {
			time.AfterFunc(time.Second, func() {
				if !checkCPUIdleDisabled(true, s) {
					s.Error("CPU Idle state not disabled during ETM collection")
				}
			})
		}

		output, sessionID, err := getPerfOutput(ctx, s, d, tc)
		if err != nil {
			s.Fatal("Failed to call GetPerfOutputV2: ", err)
		}
		defer output.Close()

		time.AfterFunc(time.Duration(stop)*time.Second, func() {
			if err := d.StopPerf(ctx, sessionID); err != nil {
				s.Fatal("Failed to call StopPerf: ", err)
			}
		})

		s.Log("Session ID: ", sessionID)
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, output); err != nil {
			s.Fatal("Failed to read perf output: ", err)
		}

		if time.Now().Sub(start) >= time.Duration(tc.durationSecs)*time.Second {
			s.Errorf("Failed to stop perf after %d seconds", stop)
		}
		// Wait 1 second to avoid race.
		testing.Sleep(ctx, time.Second)
		if tc.disableCPUIdle && !checkCPUIdleDisabled(false, s) {
			s.Error("CPU Idle state not restored after perf collection")
		}
		checkPerfData(s, buf.Bytes())
	})
}

// checkCPUIdleDisabled verifies whether all CPU's idle states match the given
// disabled status. It does not need to return any error since this is the same
// functionality implemented in the debugd perf_tool function, which will fail
// before this is called upon errors.
func checkCPUIdleDisabled(disabled bool, s *testing.State) bool {
	const cpuTopologyLocation = "/sys/devices/system/cpu/online"
	const cpuidlePathPat = "/sys/devices/system/cpu/cpu%d/cpuidle/state%d/disable"
	b, err := ioutil.ReadFile(cpuTopologyLocation)
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
				s.Errorf("%s: %s", cpuidlePath, string(disable))
				return false
			}
		}
	}
	return true
}
