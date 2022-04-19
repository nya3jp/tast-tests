// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type bootupTimes struct {
	bootType string
}

const (
	reboot       string = "reboot"
	lidCloseOpen string = "lidCloseOpen"
	powerButton  string = "powerButton"
	bootFromS5   string = "bootFromS5"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootupTimes,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Boot performance test after reboot, powerbutton and lid close open",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.arc.PerfBootService", "tast.cros.platform.BootPerfService", "tast.cros.security.BootLockboxService", "tast.cros.security.BootLockboxService"},
		Vars: []string{"servo",
			"platform.BootupTimes.bootTime",
			"platform.BootupTimes.cbmemTimeout",
			"platform.mode", // Optional. Expecting "tablet". By default platform.mode will be "clamshell".
		},
		Params: []testing.Param{{
			Name:    "reboot",
			Val:     bootupTimes{bootType: reboot},
			Timeout: 5 * time.Minute,
		}, {
			Name:    "lid_close_open",
			Val:     bootupTimes{bootType: lidCloseOpen},
			Timeout: 5 * time.Minute,
		}, {
			Name:    "power_button",
			Val:     bootupTimes{bootType: powerButton},
			Timeout: 5 * time.Minute,
		}, {
			Name:              "from_s5",
			Val:               bootupTimes{bootType: bootFromS5},
			Timeout:           5 * time.Minute,
			ExtraHardwareDeps: hwdep.D(hwdep.ChromeEC()),
		}},
	})
}

func BootupTimes(ctx context.Context, s *testing.State) {
	var (
		cbmemPattern = regexp.MustCompile(`Total Time: (.*)`)
		bootTime     = 8.4  // default bootup time in seconds
		cbmemTimeout = 1.35 // default cbmem timeout in seconds
	)
	dut := s.DUT()
	btType := s.Param().(bootupTimes)

	bootupTime, ok := s.Var("platform.BootupTimes.bootTime")
	if !ok {
		s.Log("Default Boot Time for validation: ", bootTime)
	} else {
		btime, err := strconv.ParseFloat(bootupTime, 8)
		if err != nil {
			s.Fatal("Failed to convert boot time: ", err)
		}
		bootTime = btime
		s.Log("Boot Time for validation: ", bootTime)
	}

	cbmemtime, ok := s.Var("platform.BootupTimes.cbmemTimeout")
	if !ok {
		s.Log("Default Cbmem Timeout for validation: ", cbmemTimeout)
	} else {
		cbmtime, err := strconv.ParseFloat(cbmemtime, 8)
		if err != nil {
			s.Fatal("Failed to convert cbmemtime: ", err)
		}
		cbmemTimeout = cbmtime
		s.Log("Cbmem Timeout for validation: ", cbmemTimeout)
	}
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Get the initial tablet_mode_angle settings to restore at the end of test.
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
	if err != nil {
		s.Fatal("Failed to retrieve tablet_mode_angle settings: ", err)
	}
	m := re.FindSubmatch(out)
	if len(m) != 3 {
		s.Fatalf("Failed to get initial tablet_mode_angle settings: got submatches %+v", m)
	}
	initLidAngle := m[1]
	initHys := m[2]

	defaultMode := "clamshell"
	if mode, ok := s.Var("platform.mode"); ok {
		defaultMode = mode
	}

	if defaultMode == "tablet" {
		// Set tabletModeAngle to 0 to force the DUT into tablet mode.
		testing.ContextLog(ctx, "Put DUT into tablet mode")
		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", "0", "0").Run(); err != nil {
			s.Fatal("Failed to set DUT into tablet mode: ", err)
		}
	}

	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Connect to the gRPC server on the DUT.
	// Perform a Chrome login.
	// Chrome login excluding for lidCloseOpen.
	if btType.bootType != "lidCloseOpen" {
		client := security.NewBootLockboxServiceClient(cl.Conn)
		if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to start Chrome")
		}
	}
	// Enable bootchart before running the boot perf test.
	bootPerfService := platform.NewBootPerfServiceClient(cl.Conn)
	s.Log("Enabling boot chart")
	_, err = bootPerfService.EnableBootchart(ctx, &empty.Empty{})
	if err != nil {
		// If we failed in enabling bootchart, log the failure and proceed without bootchart.
		s.Log("Warning: failed to enable bootchart: ", err)
	}
	// Stop tlsdated, that makes sure nobody will touch the RTC anymore, and also creates a sync-rtc bootstat file.
	if err := dut.Conn().CommandContext(ctx, "stop", "tlsdated").Run(); err != nil {
		s.Fatal("Failed to stop tlsdated")
	}

	// Undo the effect of enabling bootchart. This cleanup can also be performed (becomes a no-op) if bootchart is not enabled.
	defer func() {
		// Restore the side effect made in this test by disabling bootchart for subsequent system boots.
		s.Log("Disable bootchart")
		chl, err := rpc.Dial(ctx, dut, s.RPCHint())
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer chl.Close(ctx)

		bootPerfService := platform.NewBootPerfServiceClient(chl.Conn)
		_, err = bootPerfService.DisableBootchart(ctx, &empty.Empty{})
		if err != nil {
			s.Log("Error in disabling bootchart: ", err)
		}

		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", string(initLidAngle), string(initHys)).Run(); err != nil {
			s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
		}

	}()

	// Wake up DUT.
	pwrNormalPress := func() {
		s.Log("Waking up DUT")
		if !dut.Connected(ctx) {
			s.Log("Power Normal Pressing")
			waitCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
			defer cancel()
			if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Fatal("Failed to power normal press: ", err)
			}
			if err := dut.WaitConnect(waitCtx); err != nil {
				s.Fatal("Failed to wait connect DUT: ", err)
			}
		} else {
			s.Log("DUT is UP")
		}
	}

	// Cleanup.
	defer func(ctx context.Context) {
		s.Log("Performing clean up")
		pwrNormalPress()
	}(ctx)

	if btType.bootType == "reboot" {
		s.Log("Rebooting DUT")
		if err := dut.Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot DUT: ", err)
		}
	} else if btType.bootType == "lidCloseOpen" {
		s.Log("Closing lid")
		if err := pxy.Servo().SetString(ctx, "lid_open", "no"); err != nil {
			s.Fatal("Unable to close lid : ", err)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			pwrState, err := pxy.Servo().GetECSystemPowerState(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get power state S5 error")
			}
			if pwrState != "S5" {
				return errors.Errorf("System is not in S5, got: %s", pwrState)
			}
			return nil
		}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
			s.Fatal("Failed to enter S5 state : ", err)
		}
		if err := pxy.Servo().SetString(ctx, "lid_open", "yes"); err != nil {
			s.Fatal("Failed to open lid: ", err)
		}
		if err := dut.WaitConnect(ctx); err != nil {
			pwrNormalPress()
		}
	} else if btType.bootType == "powerButton" {
		if err := dut.Conn().CommandContext(ctx, "sh", "-c", "rm -rf /var/log/metrics/*").Run(); err != nil {
			s.Fatal("Failed to remove /var/log/metrics/* files: ", err)
		}
		if err := pxy.Servo().SetString(ctx, "power_key", "long_press"); err != nil {
			s.Fatal("Unable to power state off: ", err)
		}

		if err := dut.WaitUnreachable(ctx); err != nil {
			s.Fatal("Failed to shutdown: ", err)
		}
		pwrNormalPress()
	} else if btType.bootType == bootFromS5 {
		s.Log("Capturing EC log")
		if err := pxy.Servo().SetOnOff(ctx, servo.ECUARTCapture, servo.On); err != nil {
			s.Fatal("Failed to capture EC UART: ", err)
		}
		defer func() {
			if err := pxy.Servo().SetOnOff(ctx, servo.ECUARTCapture, servo.Off); err != nil {
				s.Fatal("Failed to disable capture EC UART: ", err)
			}
		}()
		var leftoverLines string
		readyForPowerOn := regexp.MustCompile(`power state 1 = S5`)
		tooLateToPowerOn := regexp.MustCompile(`power state 0 = G3`)
		powerOnFinished := regexp.MustCompile(`power state 3 = S0`)
		powerButtonPressFinished := regexp.MustCompile(`PB task 0 = idle`)
		didPowerOn := false
		hitS5 := false
		donePowerOff := false

		if err := dut.Conn().CommandContext(ctx, "sh", "-c", "rm -rf /var/log/metrics/*").Run(); err != nil {
			s.Fatal("Failed to remove /var/log/metrics/* files: ", err)
		}
		// Use the ec command here instead of power_key, because servo sleeps before the command returns
		if err := pxy.Servo().RunECCommand(ctx, "powerbtn 8500"); err != nil {
			s.Fatal("Unable to power off: ", err)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if lines, err := pxy.Servo().GetQuotedString(ctx, servo.ECUARTStream); err != nil {
				s.Fatal("Failed to read UART: ", err)
			} else if lines != "" {
				// It is possible to read partial lines, so save the part after newline for later
				lines = leftoverLines + lines
				if crlfIdx := strings.LastIndex(lines, "\r\n"); crlfIdx < 0 {
					leftoverLines = lines
					lines = ""
				} else {
					leftoverLines = lines[crlfIdx+2:]
					lines = lines[:crlfIdx+2]
				}

				for _, l := range strings.Split(lines, "\r\n") {
					s.Logf("%q", l)
					if match := readyForPowerOn.FindString(l); match != "" && !didPowerOn {
						s.Logf("Found S5: %q", l)
						hitS5 = true
					}
					if match := powerButtonPressFinished.FindString(l); match != "" && !didPowerOn {
						s.Logf("Found power button release: %q", l)
						donePowerOff = true
					}
					// If the long press above is done, and we've seen S5, then do a short press to power on.
					if hitS5 && donePowerOff && !didPowerOn {
						s.Log("Pressing power button")
						if err := pxy.Servo().SetString(ctx, servo.ECUARTCmd, "powerbtn 200"); err != nil {
							return testing.PollBreak(err)
						}
						didPowerOn = true
					}

					if match := tooLateToPowerOn.FindString(l); match != "" && !didPowerOn {
						s.Logf("Found G3: %q", l)
						s.Error("Power state reached G3, power button pressed too late")
					}
					if match := powerOnFinished.FindString(l); didPowerOn && match != "" {
						s.Logf("Found S0: %q", l)
						return nil
					}
				}
			}
			return errors.New("Not in S0 yet")
		}, &testing.PollOptions{Interval: time.Millisecond * 200, Timeout: 60 * time.Second}); err != nil {
			s.Error("EC output parsing failed: ", err)
		}
		waitCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer cancel()
		if err := dut.WaitConnect(waitCtx); err != nil {
			s.Fatal("Failed to wait connect DUT: ", err)
		}
	}
	// Validating prev sleep state for power modes.
	if btType.bootType == "reboot" {
		if err := validateSleepState(ctx, dut, 0); err != nil {
			s.Fatal("Failed to get previous sleep state: ", err)
		}
	} else {
		if err := validateSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Failed to get previous sleep state: ", err)
		}
	}

	// Validate seconds power on to login from platform bootperf values.
	getBootPerf := func() {
		cl, err := rpc.Dial(ctx, dut, s.RPCHint())
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)
		bootPerfService := platform.NewBootPerfServiceClient(cl.Conn)
		metrics, err := bootPerfService.GetBootPerfMetrics(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Failed to get boot perf metrics: ", err)
		}
		if metrics.Metrics["seconds_power_on_to_login"] > bootTime {
			s.Fatalf("Failed seconds_power_on_to_login is greater than expected, want %v; got %v", bootTime, metrics.Metrics["seconds_power_on_to_login"])
		}

	}
	getBootPerf()

	// Validating cbmem time.
	cbmemOutput, err := dut.Conn().CommandContext(ctx, "sh", "-c", "cbmem -t").Output()
	if err != nil {
		s.Fatal("Failed to execute cbmem command: ", err)
	}
	match := cbmemPattern.FindStringSubmatch(string(cbmemOutput))
	cbmemTotalTime := ""
	if len(match) > 1 {
		cbmemTotalTime = strings.Replace(match[1], ",", "", -1)
	}
	cbmemTime, _ := strconv.ParseFloat(cbmemTotalTime, 8)
	cbmemTime = cbmemTime / 1000000
	if cbmemTime > cbmemTimeout {
		s.Fatalf("Failed to validate cbmem time, actual cbmem time is more than expected cbmem time, want %v; got %v", cbmemTimeout, cbmemTime)
	}
}

// validateSleepState from cbmem command output.
func validateSleepState(ctx context.Context, dut *dut.DUT, sleepStateValue int) error {
	// Command to check previous sleep state.
	const prevSleepStateCmd = "cbmem -c | grep 'prev_sleep_state' | tail -1"
	out, err := dut.Conn().CommandContext(ctx, "sh", "-c", prevSleepStateCmd).Output()
	if err != nil {
		return err
	}
	count, err := strconv.Atoi(strings.Split(strings.Replace(string(out), "\n", "", -1), " ")[1])
	if err != nil {
		return err
	}
	if count != sleepStateValue {
		return errors.Errorf("previous sleep state must be %d", sleepStateValue)
	}
	return nil
}
