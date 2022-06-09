// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/usbutils"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type peripheralsPowerMode int

const (
	suspendTest peripheralsPowerMode = iota
	coldbootTest
)

type peripheralsTestParams struct {
	powerMode peripheralsPowerMode
	iter      int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemPeripheralsFunctionalityCheck,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies connected peripherals detection before and after power operations",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		VarDeps:      []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.InternalDisplay()),
		Params: []testing.Param{{
			Name: "suspend_quick",
			Val:  peripheralsTestParams{powerMode: suspendTest, iter: 1},
		}, {
			Name:    "suspend_bronze",
			Val:     peripheralsTestParams{powerMode: suspendTest, iter: 20},
			Timeout: 5 * time.Minute,
		}, {
			Name:    "suspend_silver",
			Val:     peripheralsTestParams{powerMode: suspendTest, iter: 50},
			Timeout: 10 * time.Minute,
		}, {
			Name:    "suspend_gold",
			Val:     peripheralsTestParams{powerMode: suspendTest, iter: 100},
			Timeout: 15 * time.Minute,
		}, {
			Name:    "coldboot_quick",
			Val:     peripheralsTestParams{powerMode: coldbootTest, iter: 1},
			Timeout: 5 * time.Minute,
		}, {
			Name:    "coldboot_bronze",
			Val:     peripheralsTestParams{powerMode: coldbootTest, iter: 20},
			Timeout: 25 * time.Minute,
		}, {
			Name:    "coldboot_silver",
			Val:     peripheralsTestParams{powerMode: coldbootTest, iter: 50},
			Timeout: 45 * time.Minute,
		}, {
			Name:    "coldboot_gold",
			Val:     peripheralsTestParams{powerMode: coldbootTest, iter: 100},
			Timeout: 85 * time.Minute,
		},
		}})
}

// SystemPeripheralsFunctionalityCheck verifies connected peripherals is detected
// or not while performing power mode operations.
// Pre-requisite: Below devices need to be connected to DUT before executing test.
// 1. USB2.0
// 2. USB3.0
// 3. SD_CARD
// 4. External HDMI display
func SystemPeripheralsFunctionalityCheck(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()
	testParam := s.Param().(peripheralsTestParams)

	servoSpec := s.RequiredVar("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power-on DUT at cleanup: ", err)
			}
		}
	}(ctxForCleanUp)

	// Perform initial Chrome login.
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to log in to Chrome: ", err)
	}

	// Check for all peripheral devices detection before suspend/cold boot.
	if err := connectedPeripheralsDetection(ctx, dut); err != nil {
		s.Fatal("Failed to detect connected peripherals devices before cold boot: ", err)
	}

	iter := testParam.iter
	switch testParam.powerMode {
	case suspendTest:
		for i := 1; i <= iter; i++ {
			s.Logf("Suspend test iteration: %d/%d", i, iter)
			powerOffCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := dut.Conn().CommandContext(powerOffCtx, "powerd_dbus_suspend").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
				s.Fatal("Failed to power off DUT: ", err)
			}

			sdCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
			defer cancel()
			if err := dut.WaitUnreachable(sdCtx); err != nil {
				s.Fatal("Failed to wait for unreachable: ", err)
			}

			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT: ", err)
			}

			// Check for all peripheral devices detection after suspend.
			if err := connectedPeripheralsDetection(ctx, dut); err != nil {
				s.Fatal("Failed to detect connected peripherals devices during suspend test: ", err)
			}
		}

	case coldbootTest:
		for i := 1; i <= iter; i++ {
			s.Logf("Cold boot test iteration: %d/%d", i, iter)
			powerState := "S5"
			if err := powercontrol.ShutdownAndWaitForPowerState(ctx, pxy, dut, powerState); err != nil {
				s.Fatalf("Failed to shutdown and wait for %q powerstate: %v", powerState, err)
			}

			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT: ", err)
			}

			// Performing chrome login after powering on DUT from cold boot.
			if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
				s.Fatal("Failed to log in to Chrome after cold boot: ", err)
			}

			// Check for all peripheral devices detection after cold boot.
			if err := connectedPeripheralsDetection(ctx, dut); err != nil {
				s.Fatal("Failed to detect connected peripherals devices after cold boot: ", err)
			}

			// Perfoming prev_sleep_state check.
			expectedPrevSleepState := 5
			if err := powercontrol.ValidatePrevSleepState(ctx, dut, expectedPrevSleepState); err != nil {
				s.Fatal("Failed to validate previous sleep state: ", err)
			}
		}
	}
}

// sdCardDetection performs SD card detection validation.
func sdCardDetection(ctx context.Context, dut *dut.DUT) error {
	const sdMmcSpecFile = "/sys/kernel/debug/mmc0/ios"
	sdCardSpecRe := regexp.MustCompile(`timing spec:.[1-9]+.\(sd.*`)
	return testing.Poll(ctx, func(ctx context.Context) error {
		isSDCardConnected := sdCardConnected(ctx, dut)
		if !isSDCardConnected {
			return errors.New("failed to find SD card")
		}
		sdCardSpecOut, err := linuxssh.ReadFile(ctx, dut.Conn(), sdMmcSpecFile)
		if err != nil {
			return errors.Wrap(err, "failed to execute sd card /sys/kernel' command")
		}
		if got := string(sdCardSpecOut); !sdCardSpecRe.MatchString(got) {
			return errors.Errorf("failed to get MMC spec info in /sys/kernel/ = got %q, want match %q", got, sdCardSpecRe)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// usbStorageDevicesDetection verifies whether connected USB storage device
// detected or not.
func usbStorageDevicesDetection(ctx context.Context, dut *dut.DUT) error {
	usbStorageClassName := "Mass Storage"
	usb2DeviceSpeed := "480M"
	usb3DeviceSpeed := "5000M"

	// Check for USB device(s) detection after cold boot.
	usbDevicesList, err := usbutils.ListDevicesInfo(ctx, dut)
	if err != nil {
		return errors.Wrap(err, "failed to get USB devices list")
	}

	usbDevicesSpeed := []string{usb2DeviceSpeed, usb3DeviceSpeed}
	for _, deviceSpeeed := range usbDevicesSpeed {
		got := usbutils.NumberOfUSBDevicesConnected(usbDevicesList, usbStorageClassName, deviceSpeeed)
		if want := 1; got != want {
			return errors.Errorf("unexpected number of USB devices connected with %q speed: got %d, want %d", deviceSpeeed, got, want)
		}
	}
	return nil
}

// connectedPeripheralsDetection verified whether all connected peripheral devices
// detected or not.
func connectedPeripheralsDetection(ctx context.Context, dut *dut.DUT) error {
	var (
		connectorInfoRe   = regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[HDMI]+.*`)
		connectedStatusRe = regexp.MustCompile(`\[CONNECTOR:\d+:HDMI.*status: connected`)
	)
	if err := usbStorageDevicesDetection(ctx, dut); err != nil {
		return errors.Wrap(err, "failed to detect connected USB storage devices")
	}
	if err := sdCardDetection(ctx, dut); err != nil {
		return errors.Wrap(err, "failed to detect connected SD Card")
	}
	numberOfDisplays := 1
	displayInfoPatterns := []*regexp.Regexp{connectorInfoRe, connectedStatusRe}
	if err := usbutils.ExternalDisplayDetection(ctx, dut, numberOfDisplays, displayInfoPatterns); err != nil {
		return errors.Wrap(err, "failed to detect external HDMI display")
	}
	return nil
}

// sdCardConnected return SD card detection status.
func sdCardConnected(ctx context.Context, dut *dut.DUT) bool {
	sdFound := false
	sysOut, err := dut.Conn().CommandContext(ctx, "ls", "/sys/block").Output()
	if err != nil {
		return sdFound
	}
	stringOut := strings.TrimSpace(string(sysOut))
	sc := bufio.NewScanner(strings.NewReader(stringOut))
	for sc.Scan() {
		sysBlockFile := fmt.Sprintf("/sys/block/%s/device/type", sc.Text())
		sdOut, err := dut.Conn().CommandContext(ctx, "cat", sysBlockFile).Output()
		if err == nil {
			if strings.TrimSpace(string(sdOut)) == "SD" {
				sdFound = true
				break
			}
		}
	}
	return sdFound
}
