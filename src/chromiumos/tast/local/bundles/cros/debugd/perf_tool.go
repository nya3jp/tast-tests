// Copyright 2022 The ChromiumOS Authors
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
	"os/exec"
	"strconv"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testCase struct {
	quipperArgs    []string // quipper arguments without the duration
	disableCPUIdle bool
	repetition     int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfTool,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests D-Bus methods related to PerfTool",
		Contacts: []string{
			"shantuo@google.com",
			"cwp-team@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "cycles",
			Val: testCase{
				quipperArgs: []string{"--", "record", "-a", "-e", "cycles", "-c", "1000003"},
				repetition:  1,
			},
			ExtraAttr: []string{"group:mainline", "informational"},
		}, {
			Name: "etm",
			Val: testCase{
				quipperArgs: []string{"--run_inject", "--inject_args", "inject;--itrace=i512il;--strip",
					"--", "record", "-e", "cs_etm/autofdo/", "-a", "-N"},
				disableCPUIdle: true,
				repetition:     1,
			},
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform("trogdor", "herobrine")),
		}, {
			Name: "etm_stress",
			Val: testCase{
				quipperArgs: []string{"--run_inject", "--inject_args", "inject;--itrace=i512il;--strip",
					"--", "record", "-e", "cs_etm/autofdo/", "-a", "-N"},
				disableCPUIdle: true,
				repetition:     100,
			},
			Timeout:           30 * time.Minute,
			ExtraAttr:         []string{"group:stress"},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform("trogdor", "herobrine")),
		}},
	})
}

const defaultDuration = 4

// PerfTool tests D-bus methods related to debugd's PerfTool.
func PerfTool(ctx context.Context, s *testing.State) {
	dbgd, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd D-Bus service: ", err)
	}

	rep := s.Param().(testCase).repetition
	if rep > 1 {
		// Stress tests run for the single call only.
		for i := 0; i < rep; i++ {
			testSingleCall(ctx, s, dbgd)
		}
	} else {
		testSingleCall(ctx, s, dbgd)
		testConsecutiveCalls(ctx, s, dbgd)
		testConcurrentCalls(ctx, s, dbgd)
		testStopEarly(ctx, s, dbgd)
		testSurviveUICrash(ctx, s, dbgd)
		testRestoreCPUIdle(ctx, s, dbgd)
	}
}

func getPerfOutput(ctx context.Context, s *testing.State, d *debugd.Debugd,
	tc testCase, durationSec int) (*os.File, uint64, error) {
	qprArgs := append([]string{"--duration", strconv.Itoa(durationSec)}, tc.quipperArgs...)

	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		s.Fatal("Failed to create status pipe: ", err)
	}
	defer wPipe.Close()

	sessionID, err := d.GetPerfOutputV2(ctx, qprArgs, tc.disableCPUIdle, wPipe)
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
	const minResultLength = 20
	s.Logf("GetPerfOutputV2() returned %d bytes of perf data", len(result))
	if len(result) < minResultLength {
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
				if err := checkCPUIdleDisabled(true); err != nil {
					s.Error("CPU Idle state not disabled during ETM collection: ", err)
				}
			})
		}

		output, sessionID, err := getPerfOutput(ctx, s, d, tc, defaultDuration)
		if err != nil {
			s.Fatal("Failed to call GetPerfOutputV2: ", err)
		}
		defer output.Close()

		s.Log("Session ID: ", sessionID)
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, output); err != nil {
			s.Fatal("Failed to read perf output: ", err)
		}
		if tc.disableCPUIdle {
			err := testing.Poll(ctx, func(_ context.Context) error {
				return checkCPUIdleDisabled(false)
			}, &testing.PollOptions{
				Timeout:  3 * time.Second,
				Interval: time.Second,
			})
			if err != nil {
				s.Error("CPU Idle state not restored after perf collection: ", err)
			}
		}
		checkPerfData(s, buf.Bytes())
	})
}

func testConsecutiveCalls(ctx context.Context, s *testing.State, d *debugd.Debugd) {
	s.Run(ctx, "testConsecutiveCalls", func(ctx context.Context, s *testing.State) {
		tc := s.Param().(testCase)
		durationSec := 1
		for i := 0; i < 3; i++ {
			output, sessionID, err := getPerfOutput(ctx, s, d, tc, durationSec)
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
				output, sessionID, err := getPerfOutput(ctx, s, d, tc, defaultDuration)
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
		durationSec := 15
		stop := 4
		start := time.Now()

		if tc.disableCPUIdle {
			time.AfterFunc(time.Second, func() {
				if err := checkCPUIdleDisabled(true); err != nil {
					s.Error("CPU Idle state not disabled during ETM collection: ", err)
				}
			})
		}

		output, sessionID, err := getPerfOutput(ctx, s, d, tc, durationSec)
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

		rt := time.Now().Sub(start)
		if rt >= time.Duration(durationSec)*time.Second {
			s.Errorf("Failed to stop perf after %d seconds", stop)
		}
		s.Log("Real perf elapsed time: ", rt)
		if tc.disableCPUIdle {
			err := testing.Poll(ctx, func(_ context.Context) error {
				return checkCPUIdleDisabled(false)
			}, &testing.PollOptions{
				Timeout:  3 * time.Second,
				Interval: time.Second,
			})
			if err != nil {
				s.Error("CPU Idle state not restored after perf collection: ", err)
			}
		}
		checkPerfData(s, buf.Bytes())
	})
}

func testSurviveUICrash(ctx context.Context, s *testing.State, d *debugd.Debugd) {
	s.Run(ctx, "testSurviveUICrash", func(ctx context.Context, s *testing.State) {
		tc := s.Param().(testCase)
		if tc.disableCPUIdle {
			time.AfterFunc(time.Second, func() {
				if err := checkCPUIdleDisabled(true); err != nil {
					s.Error("CPU Idle state not disabled during ETM collection: ", err)
				}
				cmd := exec.Command("stop", "ui")
				err := cmd.Run()
				s.Log("stop ui returned: ", err)
			})
		}

		output, sessionID, err := getPerfOutput(ctx, s, d, tc, defaultDuration)
		if err != nil {
			s.Fatal("Failed to call GetPerfOutputV2: ", err)
		}
		defer output.Close()

		s.Log("Session ID: ", sessionID)
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, output); err != nil {
			s.Fatal("Failed to read perf output: ", err)
		}
		if tc.disableCPUIdle {
			err := testing.Poll(ctx, func(_ context.Context) error {
				return checkCPUIdleDisabled(false)
			}, &testing.PollOptions{
				Timeout:  3 * time.Second,
				Interval: time.Second,
			})
			if err != nil {
				s.Error("CPU Idle state not restored after perf collection: ", err)
			}
		}
		checkPerfData(s, buf.Bytes())
	})
}

func testRestoreCPUIdle(ctx context.Context, s *testing.State, d *debugd.Debugd) {
	debugdPID := func() []byte {
		cmd := exec.Command("pgrep", "debugd")
		b, _ := cmd.Output()
		return bytes.TrimSpace(b)
	}
	killDebugd := func() []byte {
		b := debugdPID()
		cmd := exec.Command("kill", "-9", string(b))
		if err := cmd.Run(); err != nil {
			s.Fatalf("Failed to kill debugd (%s), abort: %v", string(b), err)
		}
		return b
	}
	s.Run(ctx, "testRestoreCPUIdle", func(ctx context.Context, s *testing.State) {
		tc := s.Param().(testCase)
		if !tc.disableCPUIdle {
			s.Log("Skipped, test case does not disable cpuidle states")
			return
		}
		var old []byte
		time.AfterFunc(time.Second, func() {
			if err := checkCPUIdleDisabled(true); err != nil {
				s.Error("CPU Idle state not disabled as intended: ", err)
			}
			old = killDebugd()
		})

		output, _, err := getPerfOutput(ctx, s, d, tc, defaultDuration)
		if err != nil {
			s.Fatal("Failed to call GetPerfOutputV2: ", err)
		}
		io.Copy(io.Discard, output)
		output.Close()

		err = testing.Poll(ctx, func(_ context.Context) error {
			new := debugdPID()
			if len(new) == 0 {
				return errors.New("debugd process has not respawned yet")
			}
			if bytes.Compare(new, old) == 0 {
				return errors.New("debugd process has not been killed")
			}
			return nil
		}, &testing.PollOptions{Interval: time.Second})
		if err != nil {
			s.Error("Failed to wait for debugd to respawn: ", err)
		}
		if err := checkCPUIdleDisabled(false); err != nil {
			s.Error("CPU Idle state not restored after perf collection: ", err)
		}
	})
}

// checkCPUIdleDisabled verifies whether all CPU's idle states match the given
// disabled status.
func checkCPUIdleDisabled(disabled bool) error {
	const cpuTopologyLocation = "/sys/devices/system/cpu/online"
	const cpuidlePathPat = "/sys/devices/system/cpu/cpu%d/cpuidle/state%d/disable"
	b, err := ioutil.ReadFile(cpuTopologyLocation)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", cpuTopologyLocation)
	}
	var min, max int
	if _, err := fmt.Sscanf(string(b), "%d-%d", &min, &max); err != nil {
		return errors.Wrapf(err, "unexpected CPU topology file: %s", string(b))
	}
	for cpu := min; cpu <= max; cpu++ {
		for state := 0; ; state++ {
			cpuidlePath := fmt.Sprintf(cpuidlePathPat, cpu, state)
			f, err := os.Open(cpuidlePath)
			if err != nil {
				if os.IsNotExist(err) {
					break
				}
				return errors.Wrapf(err, "failed to open %s", cpuidlePath)
			}
			defer f.Close()
			disable := make([]byte, 1)
			if n, err := f.Read(disable); err != nil || n != 1 {
				return errors.Wrapf(err, "failed to read %s", cpuidlePath)
			}
			if (disable[0] == '1') != disabled {
				return errors.Errorf("file %s shows %s, which does not match the expected state %v",
					cpuidlePath, string(disable), disabled)
			}
		}
	}
	return nil
}
