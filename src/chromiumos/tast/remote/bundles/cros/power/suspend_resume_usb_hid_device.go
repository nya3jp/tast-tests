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

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/power/powerutils"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type usbHIDTestParam struct {
	usbSpeed            string
	noOfConnectedDevice int
	powerMode           string
	usbDeviceClassName  string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SuspendResumeUSBHIDDevice,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies suspend-resume with USB HID devices functionality check",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.ScreenLockService"},
		VarDeps:      []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.X86()),
		Params: []testing.Param{{
			Name: "close_lid",
			Val: usbHIDTestParam{
				powerMode:           "closeLid",
				usbSpeed:            "1.5M",
				noOfConnectedDevice: 1,
				usbDeviceClassName:  "Human Interface Device",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "suspend_resume_command",
			Val: usbHIDTestParam{
				powerMode:           "suspendResumeCommand",
				usbSpeed:            "1.5M",
				noOfConnectedDevice: 2,
				usbDeviceClassName:  "Human Interface Device",
			},
			Timeout: 8 * time.Minute,
		},
		}})
}

func SuspendResumeUSBHIDDevice(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()
	testParam := s.Param().(usbHIDTestParam)

	servoSpec := s.RequiredVar("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	var (
		c10PackageRe = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	)

	const (
		slpS0File             = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		pkgCstateFile         = "/sys/kernel/debug/pmc_core/package_cstate_show"
		zeroPrematureWakes    = "Premature wakes: 0"
		zeroSuspendFailures   = "Suspend failures: 0"
		zeroFirmwareLogErrors = "Firmware log errors: 0"
		zeroS0ixErrors        = "s0ix errors: 0"
	)

	// Login to Chrome.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctxForCleanUp)
	screenLockService := ui.NewScreenLockServiceClient(cl.Conn)
	if _, err := screenLockService.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to login chrome: ", err)
	}
	defer screenLockService.CloseChrome(ctxForCleanUp, &empty.Empty{})

	// Check for USB Keyboard and/or Mouse detection before suspend-resume.
	if err := powerutils.USBDeviceDetection(ctx, dut, testParam.usbDeviceClassName, testParam.usbSpeed, testParam.noOfConnectedDevice); err != nil {
		s.Fatal("Failed to detect connected HID device: ", err)
	}

	slpOpSetPreBytes, err := linuxssh.ReadFile(ctx, dut.Conn(), slpS0File)
	if err != nil {
		s.Fatal("Failed to get SLP counter value: ", err)
	}

	slpOpSetPre, err := strconv.Atoi(strings.TrimSpace(string(slpOpSetPreBytes)))
	if err != nil {
		s.Fatal("Failed to convert type string to integer: ", err)
	}

	pkgOpSetOutput, err := linuxssh.ReadFile(ctx, dut.Conn(), pkgCstateFile)
	if err != nil {
		s.Fatal("Failed to get package cstate value: ", err)
	}

	matchSetPre := c10PackageRe.FindStringSubmatch(string(pkgOpSetOutput))
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
	}
	pkgOpSetPre := matchSetPre[1]

	// Context timeout for DUT unreachable and DUT wait connect.
	sdWtCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}

			testing.ContextLog(ctx, "Opening lid")
			if err := pxy.Servo().SetString(ctx, "lid_open", "yes"); err != nil {
				s.Fatal("Failed to open lid: ", err)
			}

			if err := dut.WaitConnect(ctx); err != nil {
				s.Fatal("Failed to wait connect to DUT at cleanup: ", err)
			}
		}
	}(ctxForCleanUp)

	if testParam.powerMode == "closeLid" {
		testing.ContextLog(ctx, "Closing lid")
		if err := pxy.Servo().SetString(ctx, "lid_open", "no"); err != nil {
			s.Fatal("Failed to close lid: ", err)
		}
		if err := dut.WaitUnreachable(sdWtCtx); err != nil {
			s.Fatal("Failed to wait DUT to become unreachable: ", err)
		}

		testing.ContextLog(ctx, "Opening lid")
		if err := pxy.Servo().SetString(ctx, "lid_open", "yes"); err != nil {
			s.Fatal("Failed to open lid: ", err)
		}
		if err := dut.WaitConnect(sdWtCtx); err != nil {
			s.Fatal("Failed to wait connect to DUT: ", err)
		}
	}

	if testParam.powerMode == "suspendResumeCommand" {
		// Poll for running suspend_stress_test until no premature wake and/or suspend failures occurs with given poll timeout.
		testing.ContextLog(ctx, "Wait for a suspend test without failures")
		zeroSuspendErrors := []string{zeroPrematureWakes, zeroSuspendFailures, zeroFirmwareLogErrors, zeroS0ixErrors}
		if testing.Poll(ctx, func(ctx context.Context) error {
			stressOut, err := dut.Conn().CommandContext(ctx, "suspend_stress_test", "-c", "1").Output()
			if err != nil {
				return errors.Wrap(err, "failed to execute suspend_stress_test command")
			}

			for _, errMsg := range zeroSuspendErrors {
				if !strings.Contains(string(stressOut), errMsg) {
					return errors.Errorf("expect zero failures for %q, got %q", errMsg, string(stressOut))
				}
			}
			return nil
		}, &testing.PollOptions{
			Timeout: 15 * time.Second,
		}); err != nil {
			s.Fatal("Failed to perform suspend_stress_test with zero errors: ", err)
		}
		testing.ContextLog(ctx, "Run: suspend_stress_test -c 10")
		stressOut, err := dut.Conn().CommandContext(ctx, "suspend_stress_test", "-c", "10").Output()
		if err != nil {
			s.Fatal("Failed to execute suspend_stress_test command: ", err)
		}

		for _, errMsg := range zeroSuspendErrors {
			if !strings.Contains(string(stressOut), errMsg) {
				s.Fatalf("Failed: expect zero failures for %q, got %q", errMsg, string(stressOut))
			}
		}
	}

	// Check for USB Keyboard and/or Mouse detection after suspend-resume.
	if err := powerutils.USBDeviceDetection(ctx, dut, testParam.usbDeviceClassName, testParam.usbSpeed, testParam.noOfConnectedDevice); err != nil {
		s.Fatal("Failed to detect connected HID device after suspend-resume: ", err)
	}

	slpOpSetPostBytes, err := linuxssh.ReadFile(ctx, dut.Conn(), slpS0File)
	if err != nil {
		s.Fatal("Failed to get SLP counter value after suspend-resume: ", err)
	}

	slpOpSetPost, err := strconv.Atoi(strings.TrimSpace(string(slpOpSetPostBytes)))
	if err != nil {
		s.Fatal("Failed to convert type string to integer: ", err)
	}

	if slpOpSetPre == slpOpSetPost {
		s.Fatalf("Failed: SLP counter value %q should be different from the one before suspend %q", slpOpSetPost, slpOpSetPre)
	}

	if slpOpSetPost == 0 {
		s.Fatal("Failed SLP counter value must be non-zero, got: ", slpOpSetPost)
	}

	pkgOpSetPostOutput, err := linuxssh.ReadFile(ctx, dut.Conn(), pkgCstateFile)
	if err != nil {
		s.Fatal("Failed to get package cstate value after suspend-resume: ", err)
	}

	matchSetPost := c10PackageRe.FindStringSubmatch(string(pkgOpSetPostOutput))
	if matchSetPost == nil {
		s.Fatal("Failed to match post PkgCstate value: ", pkgOpSetPostOutput)
	}

	pkgOpSetPost := matchSetPost[1]
	if pkgOpSetPre == pkgOpSetPost {
		s.Fatalf("Failed: Package C10 value %q must be different than value noted earlier %q", pkgOpSetPre, pkgOpSetPost)
	}

	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Fatal("Failed: Package C10 should be non-zero")
	}
}
