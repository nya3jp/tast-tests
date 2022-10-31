// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/remote/powercontrol"
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
	refreshPower string = "refreshPower"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootupTimes,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Boot performance test after reboot, powerbutton and lid close open",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		// Disabled due to 98%-99% failure rate and preventing other tests from running. TODO(b/242478571): fix and re-enable.
		//Attr:         []string{"group:mainline", "informational"},
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
		}, {
			Name:              "refresh_power",
			Val:               bootupTimes{bootType: refreshPower},
			Timeout:           5 * time.Minute,
			ExtraHardwareDeps: hwdep.D(hwdep.ChromeEC()),
		}},
	})
}

func BootupTimes(ctx context.Context, s *testing.State) {
	var (
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

	// Cleanup.
	defer func(ctx context.Context) {
		s.Log("Performing clean up")
		if err := powerNormalPress(ctx, dut, pxy); err != nil {
			s.Error("Failed to press power button: ", err)
		}
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
			if err := powerNormalPress(ctx, dut, pxy); err != nil {
				s.Fatal("Failed to press power button: ", err)
			}
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
		if err := powerNormalPress(ctx, dut, pxy); err != nil {
			s.Fatal("Failed to press power button: ", err)
		}
	} else if btType.bootType == bootFromS5 {
		if err := dut.Conn().CommandContext(ctx, "sh", "-c", "rm -rf /var/log/metrics/*").Run(); err != nil {
			s.Fatal("Failed to remove /var/log/metrics/* files: ", err)
		}
		// Use the ec command here instead of power_key, because servo sleeps before the command returns
		if err := pxy.Servo().RunECCommand(ctx, "powerbtn 8500"); err != nil {
			s.Fatal("Unable to power off: ", err)
		}
		if err := waitForS0State(ctx, pxy); err != nil {
			s.Fatal("Failed to wait for S0 state: ", err)
		}
		waitCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if err := dut.WaitConnect(waitCtx); err != nil {
			s.Fatal("Failed to wait connect DUT: ", err)
		}
	} else if btType.bootType == refreshPower {
		waitCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		s.Log("Pressing power btn to shutdown DUT")
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
			s.Fatal("Failed to power off DUT: ", err)
		}

		if err := dut.WaitUnreachable(ctx); err != nil {
			if err := powerNormalPress(ctx, dut, pxy); err != nil {
				s.Fatal("Failed to press power button: ", err)
			}
		}

		// expected time sleep 5 seconds to ensure dut switch to s5.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		s.Log("Pressing refresh + power key to boot up DUT")
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.Refresh, servo.DurLongPress); err != nil {
			s.Fatal("Failed to press refresh key: ", err)
		}
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			s.Fatal("Failed to power normal press: ", err)
		}
		if err := dut.WaitConnect(waitCtx); err != nil {
			s.Fatal("Failed to wait connect DUT: ", err)
		}
	}
	// Validating prev sleep state for power modes.
	if btType.bootType == "reboot" {
		if err := powercontrol.ValidatePrevSleepState(ctx, dut, 0); err != nil {
			s.Fatal("Failed to get previous sleep state: ", err)
		}
	} else {
		if err := powercontrol.ValidatePrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Failed to get previous sleep state: ", err)
		}
	}
	if err := getBootPerf(ctx, dut, s.RPCHint(), bootTime); err != nil {
		s.Fatal("Failed to get boot perf values: ", err)
	}
	if err := verifyCBMem(ctx, dut, cbmemTimeout); err != nil {
		s.Fatal("Failed to verify cbmem timeout: ", err)
	}
}

// verifyCBMem verifies cbmem timeout.
func verifyCBMem(ctx context.Context, dut *dut.DUT, cbmemTimeout float64) error {
	cbmemOutput, err := dut.Conn().CommandContext(ctx, "sh", "-c", "cbmem -t").Output()
	if err != nil {
		return errors.Wrap(err, "failed to execute cbmem command")
	}
	cbmemPattern := regexp.MustCompile(`Total Time: (.*)`)
	match := cbmemPattern.FindStringSubmatch(string(cbmemOutput))
	cbmemTotalTime := ""
	if len(match) > 1 {
		cbmemTotalTime = strings.Replace(match[1], ",", "", -1)
	}
	cbmemTime, _ := strconv.ParseFloat(cbmemTotalTime, 8)
	cbmemTime = cbmemTime / 1000000
	if cbmemTime > cbmemTimeout {
		return errors.Wrapf(err, "failed to validate cbmem time, actual cbmem time is more than expected cbmem time, want %v; got %v", cbmemTimeout, cbmemTime)
	}
	return nil
}

// getBootPerf validates seconds power on to login from platform bootperf values.
func getBootPerf(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, btime float64) error {
	cl, err := rpc.Dial(ctx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	bootPerfService := platform.NewBootPerfServiceClient(cl.Conn)
	metrics, err := bootPerfService.GetBootPerfMetrics(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "failed to get boot perf metrics")
	}
	if metrics.Metrics["seconds_power_on_to_login"] > btime {
		return errors.Wrapf(err, "failed seconds_power_on_to_login is greater than expected, want %v; got %v", btime, metrics.Metrics["seconds_power_on_to_login"])
	}
	return nil
}

// powerNormalPress wakes up DUT by normal pressing power button.
func powerNormalPress(ctx context.Context, dut *dut.DUT, pxy *servo.Proxy) error {
	testing.ContextLog(ctx, "Waking up DUT")
	if !dut.Connected(ctx) {
		testing.ContextLog(ctx, "Power Normal Pressing")
		waitCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to power normal press")
		}
		if err := dut.WaitConnect(waitCtx); err != nil {
			return errors.Wrap(err, "failed to wait connect DUT")
		}
	} else {
		testing.ContextLog(ctx, "DUT is UP")
	}
	return nil
}

// waitForS0State waits for S0 power state
func waitForS0State(ctx context.Context, pxy *servo.Proxy) error {
	var leftoverLines string
	readyForPowerOn := regexp.MustCompile(`power state 1 = S5`)
	tooLateToPowerOn := regexp.MustCompile(`power state 0 = G3`)
	powerOnFinished := regexp.MustCompile(`power state 3 = S0`)
	powerButtonPressFinished := regexp.MustCompile(`PB task 0 = idle`)
	didPowerOn := false
	hitS5 := false
	donePowerOff := false
	testing.ContextLog(ctx, "Capturing EC log")
	if err := pxy.Servo().SetOnOff(ctx, servo.ECUARTCapture, servo.On); err != nil {
		return errors.Wrap(err, "failed to capture EC UART")
	}
	defer func() error {
		if err := pxy.Servo().SetOnOff(ctx, servo.ECUARTCapture, servo.Off); err != nil {
			return errors.Wrap(err, "failed to disable capture EC UART")
		}
		return nil
	}()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		lines, err := pxy.Servo().GetQuotedString(ctx, servo.ECUARTStream)
		if err != nil {
			return errors.Wrap(err, "failed to read UART")
		}
		if lines == "" {
			return errors.New("Not in S0 yet")
		}
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
			testing.ContextLogf(ctx, "%q", l)
			if readyForPowerOn.MatchString(l) && !didPowerOn {
				testing.ContextLogf(ctx, "Found S5: %q", l)
				hitS5 = true
			}
			if powerButtonPressFinished.MatchString(l) && !didPowerOn {
				testing.ContextLogf(ctx, "Found power button release: %q", l)
				donePowerOff = true
			}
			// If the long press above is done, and we've seen S5, then do a short press to power on.
			if hitS5 && donePowerOff && !didPowerOn {
				testing.ContextLog(ctx, "Pressing power button")
				if err := pxy.Servo().SetString(ctx, servo.ECUARTCmd, "powerbtn 200"); err != nil {
					return testing.PollBreak(err)
				}
				didPowerOn = true
			}

			if tooLateToPowerOn.MatchString(l) && !didPowerOn {
				testing.ContextLogf(ctx, "Found G3: %q", l)
				return errors.New("power state reached G3, power button pressed too late")
			}
			if powerOnFinished.MatchString(l) && didPowerOn {
				testing.ContextLogf(ctx, "Found S0: %q", l)
				return nil
			}
		}
		return nil
	}, &testing.PollOptions{Interval: 200 * time.Millisecond, Timeout: time.Minute}); err != nil {
		return errors.Wrap(err, "EC output parsing failed")
	}
	return nil
}
