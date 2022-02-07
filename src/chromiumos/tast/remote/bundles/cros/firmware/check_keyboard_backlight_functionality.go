// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckKeyboardBacklightFunctionality,
		Desc:         "Confirm keyboard backlight support and check keyboard backlight functionality",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.ChromeUIService"},
		HardwareDeps: hwdep.D(
			hwdep.ChromeEC(),
			hwdep.KeyboardBacklight(),
		),
		Fixture: fixture.NormalMode,
	})
}

// CheckKeyboardBacklightFunctionality confirms keyboard backlight support and verifies its functionality.
func CheckKeyboardBacklightFunctionality(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	if err := h.RequireRPCClient(ctx); err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	serviceClient := pb.NewChromeUIServiceClient(h.RPCClient.Conn)
	if _, err := serviceClient.EnsureLoginScreen(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to restart ui at login screen: ", err)
	}

	initValue, err := checkInitKBBacklight(ctx, h)
	if err != nil {
		s.Fatal("Failed to check initial keybaord backlight value: ", err)
	}
	if initValue == 0 {
		s.Log("Keyboard initial backlight value is 0, attempting to increase the light to at least 40 percent before test")
		if err := adjustKBBacklight(ctx, s, h, 40, "<f7>", "increasing"); err != nil {
			s.Fatal("Failed to adjust keyboard backlight: ", err)
		}
	} else if initValue == 100 {
		s.Log("Keyboard initial backlight value is 100, attempting to decrease the light to at leaset 40 percent before test")
		if err := adjustKBBacklight(ctx, s, h, 40, "<f6>", "decreasing"); err != nil {
			s.Fatal("Failed to adjust keyboard backlight: ", err)
		}
	}

	kbBacklightTesting := make(map[int]string, 2)
	kbBacklightTesting[0] = "<f6>"
	kbBacklightTesting[100] = "<f7>"

	for extremeValue, key := range kbBacklightTesting {
		s.Logf("-----Adjusting keyboard backlight till %d percent-----", extremeValue)
		if err := adjustKBBacklight(ctx, s, h, extremeValue, key, ""); err != nil {
			s.Fatal("Failed to adjust keyboard backlight: ", err)
		}
	}
}

func checkInitKBBacklight(ctx context.Context, h *firmware.Helper) (int, error) {
	// Press on a key and check the initial keyboard brightness value.
	if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurPress); err != nil {
		return 0, errors.Wrap(err, "failed to press ENTER to check initial kb backlight")
	}
	// Delay by 1 second to wait for keyboard to be lit up.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return 0, errors.Wrap(err, "error in sleeping for 1 second after pressing on the ENTER key")
	}

	kbLight, err := h.Servo.GetKBBacklight(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get kb backlight")
	}

	return kbLight, nil
}

func shouldContinue(kbBacklight, extremeValue int, action string) bool {
	switch action {
	case "increasing":
		return kbBacklight <= extremeValue
	case "decreasing":
		return kbBacklight >= extremeValue
	default:
		return kbBacklight != extremeValue
	}
}

func adjustKBBacklight(ctx context.Context, s *testing.State, h *firmware.Helper, extremeValue int, actionKey, action string) error {
	kbLight, err := h.Servo.GetKBBacklight(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get kb backlight")
	}
	for shouldContinue(kbLight, extremeValue, action) {
		s.Logf("Attempting to match, current: %d, expected: %d", kbLight, extremeValue)
		if err := pressShortcut(ctx, h, actionKey); err != nil {
			return errors.Wrap(err, "failed to adjust kb backlight brightness")
		}
		kbLight, err = h.Servo.GetKBBacklight(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get kb backlight")
		}
	}
	return nil
}

func pressShortcut(ctx context.Context, h *firmware.Helper, actionKey string) error {
	// ShortCuts for decreasing keyboard backlight: Alt+F6 (Alt+BrightnessDown).
	// ShortCuts for increasing keyboard backlight: Alt+F7 (Alt+BrightnessUp).
	row, col, err := h.Servo.GetKeyRowCol("<alt_l>")
	if err != nil {
		return errors.Wrap(err, "failed to get key column and row")
	}

	altHold := fmt.Sprintf("kbpress %d %d 1", col, row)
	altRelease := fmt.Sprintf("kbpress %d %d 0", col, row)

	if err := h.Servo.RunECCommand(ctx, altHold); err != nil {
		return errors.Wrap(err, "failed to press and hold alt")
	}
	if err := h.Servo.ECPressKey(ctx, actionKey); err != nil {
		return errors.Wrapf(err, "failed to press %q", actionKey)
	}
	if err := h.Servo.RunECCommand(ctx, altRelease); err != nil {
		return errors.Wrap(err, "failed to release alt")
	}
	return nil
}
