// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"strconv"
	"strings"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

// Note: Pendrive connected to the servo should be having the recovery OS flashed.

func init() {
	testing.AddTest(&testing.Test{
		Func:         RecModeTime,
		Desc:         "Test to ensure boot to recovery mode time is consistently less than 5 seconds",
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Fixture:      fixture.DevMode,
		Timeout:      10 * time.Minute,
	})
}

func RecModeTime(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	s.Log("Stopping power supply")
	if err := h.SetDUTPower(ctx, false); err != nil {
		s.Fatal("Failed to remove charger: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
			return errors.Wrap(err, "error checking whether charger is attached")
		} else if attached {
			return errors.New("charger is still attached - use Servo V4 Type-C or supply RPM vars")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to check if charger is disconnected via Servo: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		if err := h.SetDUTPower(ctx, true); err != nil {
			s.Fatal("Failed to attach charger: ", err)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
				return errors.Wrap(err, "error checking whether charger is attached")
			} else if !attached {
				return errors.New("charger is not attached at cleanup - use Servo V4 Type-C or supply RPM vars")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Failed to check if charger is connected via Servo: ", err)
		}
	}(cleanupCtx)

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create new boot mode switcher: ", err)
	}
	s.Log("Rebooting into recovery mode")
	if err := ms.RebootToMode(ctx, fwCommon.BootModeRecovery); err != nil {
		s.Fatal("Failed to reboot into recovery mode: ", err)
	}

	s.Log("Reconnecting to DUT")
	if err := h.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	s.Log("Checking that DUT has booted from removable device")
	bootedFromRemovableDevice, err := h.Reporter.BootedFromRemovableDevice(ctx)
	if err != nil {
		s.Fatal("Failed to determine boot device type: ", err)
	}
	if !bootedFromRemovableDevice {
		s.Fatalf("DUT did not boot from the bootable device: got %v, want true", bootedFromRemovableDevice)
	}

	fwBootTime, err := firmwareTimestampBootTime(ctx, h)
	if err != nil {
		s.Fatal("Failed to read firmware boot time: ", err)
	}

	const expectedBootTimeSec float64 = 5
	if fwBootTime > expectedBootTimeSec {
		s.Fatalf("Failed to verify firmware boot time, got: %.2f, want: <=%.2f sec", fwBootTime, expectedBootTimeSec)
	}

}

// firmwareTimestampBootTime reads firmware startup time from /tmp/firmware-boot-time
// and returns the value in seconds.
func firmwareTimestampBootTime(ctx context.Context, h *firmware.Helper) (float64, error) {
	b, err := h.Reporter.CatFile(ctx, "/tmp/firmware-boot-time")
	for err != nil {
		return 0, errors.Wrap(err, "failed to read firmware-boot-time file")
	}
	l := strings.Split(string(b), "\n")[0]
	return strconv.ParseFloat(l, 64)
}
