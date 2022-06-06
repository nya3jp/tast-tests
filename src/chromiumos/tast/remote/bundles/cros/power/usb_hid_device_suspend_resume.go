// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/usbutils"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type powerMode int

const (
	closeLid powerMode = iota
	suspendResumeCommand
)

type usbHIDTestParam struct {
	powerMode                powerMode
	usbDeviceClassName       string
	usbSpeed                 string
	numberOfConnectedDevices int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBHIDDeviceSuspendResume,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies USB HID devices functionality with suspend-resume",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.ScreenLockService"},
		VarDeps:      []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.X86()),
		Params: []testing.Param{{
			Name:    "close_lid",
			Val:     usbHIDTestParam{closeLid, "Human Interface Device", "1.5M", 1},
			Timeout: 5 * time.Minute,
		}, {
			Name:    "suspend_resume_command",
			Val:     usbHIDTestParam{suspendResumeCommand, "Human Interface Device", "1.5M", 2},
			Timeout: 8 * time.Minute,
		}},
	})
}

func USBHIDDeviceSuspendResume(ctx context.Context, s *testing.State) {
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

	// Login to Chrome.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctxForCleanUp)
	screenLockService := ui.NewScreenLockServiceClient(cl.Conn)
	if _, err := screenLockService.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to log in to chrome: ", err)
	}
	defer screenLockService.CloseChrome(ctxForCleanUp, &empty.Empty{})

	// Check for USB Keyboard and/or Mouse detection before suspend-resume.
	usbDevicesList, err := usbutils.ListDevicesInfo(ctx, dut)
	if err != nil {
		s.Fatal("Failed to get USB devices list: ", err)
	}

	got := usbutils.NumberOfUSBDevicesConnected(usbDevicesList, testParam.usbDeviceClassName, testParam.usbSpeed)
	if want := testParam.numberOfConnectedDevices; got != want {
		s.Fatalf("Unexpected number of USB devices connected: got %d, want %d", got, want)
	}

	slpOpSetPre, pkgOpSetPre, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values before suspend-resume: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}

			s.Log("Opening lid")
			if err := pxy.Servo().SetString(ctx, "lid_open", "yes"); err != nil {
				s.Fatal("Failed to open lid: ", err)
			}

			if err := dut.WaitConnect(ctx); err != nil {
				s.Fatal("Failed to wait connect to DUT at cleanup: ", err)
			}
		}
	}(ctxForCleanUp)

	switch testParam.powerMode {
	case closeLid:
		s.Log("Closing lid")
		if err := pxy.Servo().SetString(ctx, "lid_open", "no"); err != nil {
			s.Fatal("Failed to close lid: ", err)
		}
		// Context timeout for DUT unreachable and DUT wait connect.
		sdWtCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
		defer cancel()
		if err := dut.WaitUnreachable(sdWtCtx); err != nil {
			s.Fatal("Failed to wait DUT to become unreachable: ", err)
		}

		s.Log("Opening lid")
		if err := pxy.Servo().SetString(ctx, "lid_open", "yes"); err != nil {
			s.Fatal("Failed to open lid: ", err)
		}
		if err := dut.WaitConnect(sdWtCtx); err != nil {
			s.Fatal("Failed to wait connect to DUT: ", err)
		}

	case suspendResumeCommand:
		suspendStressTestCounter := 10
		if err := powercontrol.PerformSuspendStressTest(ctx, dut, suspendStressTestCounter); err != nil {
			s.Fatal("Failed to perform suspend stress test: ", err)
		}
	}

	// Check for USB Keyboard and/or Mouse detection after suspend-resume.
	usbDevicesList, err = usbutils.ListDevicesInfo(ctx, dut)
	if err != nil {
		s.Fatal("Failed to get USB devices list: ", err)
	}

	got = usbutils.NumberOfUSBDevicesConnected(usbDevicesList, testParam.usbDeviceClassName, testParam.usbSpeed)
	if want := testParam.numberOfConnectedDevices; got != want {
		s.Fatalf("Unexpected number of USB devices connected: got %d, want %d", got, want)
	}

	slpOpSetPost, pkgOpSetPost, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values after suspend-resume: ", err)
	}

	if slpOpSetPre == slpOpSetPost {
		s.Fatalf("Failed: SLP counter value %q should be different from the one before suspend %q", slpOpSetPost, slpOpSetPre)
	}

	if slpOpSetPost == 0 {
		s.Fatal("Failed SLP counter value must be non-zero, got: ", slpOpSetPost)
	}

	if pkgOpSetPre == pkgOpSetPost {
		s.Fatalf("Failed: Package C10 value %q must be different from the one before suspend %q", pkgOpSetPost, pkgOpSetPre)
	}

	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Fatal("Failed: Package C10 should be non-zero")
	}
}
