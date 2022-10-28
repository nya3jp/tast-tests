// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type wakeECMethod string

const (
	ecrstWakeECMethod     wakeECMethod = "ecrst"
	coldResetWakeECMethod wakeECMethod = "cold_reset"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CR50ECReset,
		Desc:         "Make sure 'cr50 ecrst' works as intended. EC_RST_L needs to be able to wake the EC from hibernate and hold the EC in reset",
		Contacts:     []string{"pf@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.NormalMode,
		Timeout:      25 * time.Minute,
		Data:         []string{firmware.ConfigFile},
		SoftwareDeps: []string{"gsc"},
		// Skip model in which a flag gsc_can_wake_ec_with_reset is false
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.SkipOnModel("grunt", "nami")),
	})
}

func CR50ECReset(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}

	if !h.Config.GSCCanWakeECWithReset {
		s.Fatal("This DUT has a hardware limitation that prevents cr50 from waking the EC with EC_RST_L. Exclude the Model from this test by adding it to HW Dep")
	}

	if !h.Config.Hibernate {
		s.Fatal("Failed to hibernate, because this DUT doesn't support it. Exclude the Model from this test by adding it to HW Dep")
	}

	servoType, err := h.Servo.GetServoType(ctx)
	if err != nil {
		s.Fatal("Failed to get servo type: ", err)
	}

	if strings.Contains(servoType, "c2d2") {
		s.Fatal("Cannot run test with c2d2")
	}

	if err := h.OpenCCD(ctx, true, true); err != nil {
		s.Fatal("Failed to open CCD: ", err)
	}

	if err := basicEcrstTest(ctx, h); err != nil {
		s.Fatal("Failed to perform basic ecrst test: ", err)
	}

	if err := wakeEC(ctx, h, ecrstWakeECMethod); err != nil {
		s.Fatal("Failed to wake EC by using 'ecrst': ", err)
	}

	if err := wakeEC(ctx, h, coldResetWakeECMethod); err != nil {
		s.Fatal("Failed to wake EC by using 'ecrst': ", err)
	}
}

// wakeEC will check if given `method` can wake EC form hibernate
func wakeEC(ctx context.Context, h *firmware.Helper, method wakeECMethod) error {
	if err := h.Servo.ECHibernate(ctx, servo.UseConsole); err != nil {
		errors.Wrap(err, "failed to hibernate DUT via EC command")
	}
	if method == coldResetWakeECMethod {
		if err := h.Servo.SetOnOff(ctx, servo.ColdReset, servo.On); err != nil {
			errors.Wrapf(err, "failed to set %s to on", method)
		}
		if err := h.Servo.SetOnOff(ctx, servo.ColdReset, servo.Off); err != nil {
			errors.Wrapf(err, "failed to set %s to off", method)
		}
	} else {
		if err := h.Servo.RunCR50Command(ctx, "ecrst on"); err != nil {
			errors.Wrapf(err, "failed to set %s to on", method)
		}
		if err := h.Servo.RunCR50Command(ctx, "ecrst off"); err != nil {
			errors.Wrapf(err, "failed to set %s to off", method)
		}
	}
	if err := h.Servo.CheckUnresponsiveEC(ctx); err == nil {
		errors.Wrapf(err, "failed to release the EC from reset using %s", method)
	}

	if err := guaranteeECIsUp(ctx, h); err != nil {
		errors.Wrap(err, "failed to guarantee that EC is up")
	}

	return nil
}

// basicEcrstTest will check ability to connect to an EC console when it is in reset state brought by ecrst
func basicEcrstTest(ctx context.Context, h *firmware.Helper) error {
	if err := h.Servo.RunCR50Command(ctx, "ecrst on"); err != nil {
		return errors.Wrap(err, "failed to send a `ecrst on` command to CR50")
	}
	if err := h.Servo.CheckUnresponsiveEC(ctx); err != nil {
		return errors.New("failed to use cr50 ecrst to hold the EC in reset")
	}
	if err := h.Servo.RunCR50Command(ctx, "ecrst off"); err != nil {
		return errors.Wrap(err, "failed to send a `ecrst off` command to CR50")
	}
	if err := h.Servo.CheckUnresponsiveEC(ctx); err == nil {
		return errors.New("failed to release the EC from reset")
	}

	if err := guaranteeECIsUp(ctx, h); err != nil {
		return errors.Wrap(err, "failed to guarantee that EC is up")
	}

	return nil
}

// guaranteeECIsUp make sure that EC isn't held in reset. It uses the power button to wake EC
func guaranteeECIsUp(ctx context.Context, h *firmware.Helper) error {
	if err := h.Servo.SetOnOff(ctx, servo.ColdReset, servo.Off); err != nil {
		return errors.Wrap(err, "failed to set cold_reset to off")
	}
	if err := h.Servo.RunCR50Command(ctx, "ecrst off"); err != nil {
		return errors.Wrap(err, "failed to send a `ecrst off` command to CR50")
	}
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
		return errors.Wrap(err, "failed to press power key on DUT in order to wake EC")
	}
	if err := h.Servo.CheckUnresponsiveEC(ctx); err == nil {
		return errors.New("failed to recover EC with power button")
	}

	return nil
}
