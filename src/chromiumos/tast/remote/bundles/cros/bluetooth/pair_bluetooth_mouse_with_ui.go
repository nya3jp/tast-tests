// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/remote/bluetooth"
	"chromiumos/tast/remote/chrome/uiauto/quicksettings"
	quicksettingsService "chromiumos/tast/services/cros/chrome/uiauto/quicksettings"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PairBluetoothMouseWithUI,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that we can pair a Bluetooth mouse with the UI",
		Contacts: []string{
			"chadduffin@google.com",
			"cros-connectivity@google.com",
		},
		// TODO(b/245584709): Need to make new btpeer test attributes.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps: []string{
			"tast.cros.bluetooth.BluetoothUIService",
			"tast.cros.chrome.uiauto.quicksettings.QuickSettingsService",
			"tast.cros.inputs.KeyboardService",
			"tast.cros.ui.AutomationService",
		},
		Fixture: "chromeLoggedInWith1BTPeer",
		Timeout: time.Second * 30,
		Params: []testing.Param{{
			Name:  "mouse",
			Value: cbt.DeviceTypeMouse,
		}, {
			Name:  "keyboard",
			Value: cbt.DeviceTypeKeyboard,
		}},
	})
}

func createEmulatedDevice(ctx context.Context, deviceType cbt.DeviceType) (EmulatedBTPeerDevice, error) {
	var emulatedDevice EmulatedBTPeerDevice
	var err error
	if deviceType == cbt.DeviceTypeMouse {
		emulatedDevice, err = bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[0].BluetoothMouseDevice())
	} else if deviceType == cbt.DeviceTypeKeyboard {
		emulatedDevice, err = bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[0].BluetoothKeyboardDevice())
	} else {
		return nil, errors.New("Device type must be mouse or keyboard")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize the device")
	}
	if emulatedDevice.DeviceType() != deviceType {
		return nil, errors.New("Unexpected device type, got: " + emulatedDevice.DeviceType() + "want: " + deviceType)
	}
	return emulatedDevice, nil
}

func PairBluetoothMouseWithUI(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	if len(fv.BTPeers[0]) != 1 {
		s.Fatalf("Unexpected number of peer devices available, got: %d, want: %d", len(fv.BTPeers[0]), 1)
	}

	// Initialize the device.
	emulatedDevice, err := createEmulatedDevice(ctx, s.Param().(cbt.DeviceType))
	if err != nil {
		s.Fatal("Failed to emulate the device type: ", err)
	}

	qs := quicksettingsService.NewQuickSettingsServiceClient(fv.DUTRPCClient)
	uiautomation := ui.NewAutomationServiceClient(fv.DUTRPCClient)

	if _, err = qs.NavigateToBluetoothDetailedView(ctx, &emptypb.Empty{}); err != nil {
		s.Error("Failed to navigate to the detailed Bluetooth within Quick Settings: ", err)
	}

	if _, err := uiautomation.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: quicksettings.PairNewDeviceButton}); err != nil {
		s.Fatal("Failed to find the network button: ", err)
	}
	if _, err := uiautomation.LeftClick(
		ctx, &ui.LeftClickRequest{Finder: quicksettings.PairNewDeviceButton}); err != nil {
		s.Fatal("Failed to click the network button: ", err)
	}
	if _, err := uiautomation.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: quicksettings.PairNewDeviceDialog}); err != nil {
		s.Fatal("Failed to find the pairing dialog: ", err)
	}

	emulatedDeviceName := emulatedDevice.AsDeviceInfo().Name
	emulatedDeviceFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_NameContaining{NameContaining: emulatedDeviceName}},
			{Value: &ui.NodeWith_Ancestor{Ancestor: quicksettings.PairNewDeviceDialog}},
		},
	}

	if _, err := uiautomation.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: emulatedDeviceFinder}); err != nil {
		s.Fatal("Failed to find the emulated mouse in the pairing dialog: ", err)
	}
	if _, err := uiautomation.LeftClick(
		ctx, &ui.LeftClickRequest{Finder: emulatedDeviceFinder}); err != nil {
		s.Fatal("Failed to click the emulated mouse in the pairing dialog: ", err)
	}

	if pinCode, err := emulatedDevice.RPC.GetPinCode(ctx); err != nil {
		s.Fatal("Failed to check if the emulated peripheral has a pin code: ", err)
	} else if len(pinCode) > 0 {
		kb := inputs.NewKeyboardServiceClient(fv.DUTRPCClient)

		if _, err := kb.Type(ctx, &inputs.TypeRequest{
			Key: pinCode,
		}); err != nil {
			s.Fatal("Failed to enter the pin code when pairing: ", err)
		}
		if _, err := kb.Accel(ctx, &inputs.AccelRequest{
			Key: "Enter",
		}); err != nil {
			s.Fatal("Failed to press enter after entering the pin code when pairing: ", err)
		}
	}

	if _, err := uiautomation.WaitUntilGone(
		ctx, &ui.WaitUntilGoneRequest{Finder: quicksettings.PairNewDeviceDialog}); err != nil {
		s.Fatal("Failed to wait for the pairing dialog to be gone: ", err)
	}

	toastFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_NameContaining{NameContaining: emulatedDeviceName + " connected"}},
			{Value: &ui.NodeWith_HasClass{HasClass: "ToastOverlay"}},
		},
	}

	if _, err := uiautomation.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: toastFinder}); err != nil {
		s.Fatal("Failed to find the toast shown after successfully pairing: ", err)
	}
}
