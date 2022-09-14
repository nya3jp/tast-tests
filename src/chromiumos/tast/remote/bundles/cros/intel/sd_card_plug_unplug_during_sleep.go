// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	// TODO(b/238157101): We are not running this test on any bots intentionally.
	// ambalavanan.m.m@intel.com needs to add this test as part of a new suite.
	testing.AddTest(&testing.Test{
		Func:         SDCardPlugUnplugDuringSleep,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies microSD card functionality with plug and unplug during sleep",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"servo", "intel.sdCardDetectionName"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.X86()),
		Params: []testing.Param{{
			Name:    "plug",
			Val:     true,
			Timeout: 5 * time.Minute,
		}, {
			Name:    "unplug",
			Val:     false,
			Timeout: 5 * time.Minute,
		},
		}})
}

// SDCardPlugUnplugDuringSleep checks microSD card functionality with plug
// and unplug during sleep.
// Pre-requisite: SD card must be inserted into servo microSD card slot.
func SDCardPlugUnplugDuringSleep(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()

	sdCardName := s.RequiredVar("intel.sdCardDetectionName")
	servoSpec := s.RequiredVar("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Error("Failed to power-on DUT at cleanup: ", err)
			}
		}
		if err := unplugSDCardViaServo(ctx, pxy); err != nil {
			s.Error("Failed to unplug microSD card as cleanup: ", err)
		}
	}(cleanupCtx)

	// Performs Chrome login.
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login Chrome: ", err)
	}

	slpOpSetPre, pkgOpSetPre, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values before suspend-resume: ", err)
	}

	if err := plugSDCardViaServo(ctx, pxy); err != nil {
		s.Fatal("Failed to plug microSD storage device to DUT: ", err)
	}

	if err := waitForSDCardDetection(ctx, dut, sdCardName); err != nil {
		s.Fatal("Failed to wait for microSD card detection: ", err)
	}

	powerOffCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := dut.Conn().CommandContext(powerOffCtx, "powerd_dbus_suspend")
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to suspend DUT with powerd_dbus_suspend command: ", err)
	}

	sdCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		s.Fatal("Failed to wait for unreachable: ", err)
	}

	// Plug/unplug microSD storage device while DUT in sleep.
	isPlug := s.Param().(bool)
	if isPlug {
		s.Log("Plugging microSD after suspend")
		if err := plugSDCardViaServo(ctx, pxy); err != nil {
			s.Fatal("Failed to plug microSD storage device to DUT: ", err)
		}
	} else {
		s.Log("Unplugging microSD after suspend")
		if err := plugSDCardViaServo(ctx, pxy); err != nil {
			s.Fatal("Failed to unplug microSD storage device from DUT: ", err)
		}
	}

	sdCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		s.Fatal("Failed: DUT is not unreachable after plug/unplug of microSD: ", err)
	}

	s.Log("Waking DUT with power button normal press via servo")
	if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
		s.Fatal("Failed to wake DUT at cleanup: ", err)
	}

	slpOpSetPost, pkgOpSetPost, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values after lid-open: ", err)
	}

	if slpOpSetPre == slpOpSetPost {
		s.Fatalf("Failed: SLP counter value %v should be different from the one before %v", slpOpSetPost, slpOpSetPre)
	}
	if slpOpSetPost == 0 {
		s.Fatal("Failed: SLP counter value must be non-zero, got: ", slpOpSetPost)
	}

	if pkgOpSetPre == pkgOpSetPost {
		s.Fatalf("Failed: Package C10 value %q must be different from the one before %q", pkgOpSetPost, pkgOpSetPre)
	}
	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Fatal("Failed: Package C10 should be non-zero, got: ", pkgOpSetPost)
	}

	if isPlug {
		if err := waitForSDCardDetection(ctx, dut, sdCardName); err != nil {
			s.Fatal("Failed to wait for microSD card detection after plug: ", err)
		}
	}
}

// plugSDCardViaServo perform microSD plug via servo.
func plugSDCardViaServo(ctx context.Context, pxy *servo.Proxy) error {
	if err := pxy.Servo().SetString(ctx, "sd_en", "on"); err != nil {
		return errors.Wrap(err, "failed to set sd_en to on")
	}
	if err := pxy.Servo().SetString(ctx, "sd_pwr_en", "on"); err != nil {
		return errors.Wrap(err, "failed to set sd_pwr_en to on")
	}
	if err := pxy.Servo().SetString(ctx, "sd_mux_sel", "dut_sees_usbkey"); err != nil {
		return errors.Wrap(err, "failed to set sd_mux_sel to dut_sees_usbkey")
	}

	return nil
}

// unplugSDCardViaServo perform microSD unplug via servo.
func unplugSDCardViaServo(ctx context.Context, pxy *servo.Proxy) error {
	if err := pxy.Servo().SetString(ctx, "sd_en", "off"); err != nil {
		return errors.Wrap(err, "failed to set sd_en to off")
	}
	if err := pxy.Servo().SetString(ctx, "sd_pwr_en", "off"); err != nil {
		return errors.Wrap(err, "failed to set sd_pwr_en to off")
	}

	return nil
}

// waitForSDCardDetection waits for connected microSD card to detect with provided sdCardName.
func waitForSDCardDetection(ctx context.Context, dut *dut.DUT, sdCardName string) error {
	mediaRemovablePath := "/media/removable"
	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := dut.Conn().CommandContext(ctx, "ls", mediaRemovablePath).Output()
		if err != nil {
			return errors.Wrap(err, "failed to find connect microSD card in ls command")
		}
		if !strings.Contains(string(out), sdCardName) {
			return errors.New("failed to find connected microSD card in ls command")
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}
