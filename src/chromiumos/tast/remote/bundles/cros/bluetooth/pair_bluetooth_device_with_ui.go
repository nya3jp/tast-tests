// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/common/chameleon"
	cbt "chromiumos/tast/common/chameleon/devices/common/bluetooth"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bluetooth"
	"chromiumos/tast/remote/chrome/uiauto/quicksettings"
	quicksettingsService "chromiumos/tast/services/cros/chrome/uiauto/quicksettings"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PairBluetoothDeviceWithUI,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that we can pair a Bluetooth device with the UI",
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
			"tast.cros.ui.ChromeUIService",
		},
		Fixture: "chromeLoggedInWith1BTPeer",
		Timeout: time.Second * 30,
		Params: []testing.Param{{
			Name: "keyboard",
			Val:  cbt.DeviceTypeKeyboard,
		}, {
			Name: "mouse",
			Val:  cbt.DeviceTypeMouse,
		}},
	})
}

func createEmulatedDevice(ctx context.Context, c chameleon.Chameleond, deviceType cbt.DeviceType) (*bluetooth.EmulatedBTPeerDevice, error) {
	var emulatedDevice *bluetooth.EmulatedBTPeerDevice
	var err error

	if deviceType == cbt.DeviceTypeMouse {
		emulatedDevice, err = bluetooth.NewEmulatedBTPeerDevice(ctx, c.BluetoothMouseDevice())
	} else if deviceType == cbt.DeviceTypeKeyboard {
		emulatedDevice, err = bluetooth.NewEmulatedBTPeerDevice(ctx, c.BluetoothKeyboardDevice())
	} else {
		return nil, errors.New("Device type must be mouse or keyboard")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize the device")
	}
	if emulatedDevice.DeviceType() != deviceType {
		return nil, errors.New("Unexpected device type, got: " + emulatedDevice.DeviceType().String() + "want: " + deviceType.String())
	}
	if err = emulatedDevice.RPC().SetDiscoverable(ctx, true); err != nil {
		return nil, errors.Wrap(err, "failed to make device discoverable")
	}
	return emulatedDevice, nil
}

func PairBluetoothDeviceWithUI(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	if len(fv.BTPeers) != 1 {
		s.Fatalf("Unexpected number of peer devices available, got: %d, want: %d", len(fv.BTPeers), 1)
	}

	// Reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	// Initialize the device.
	emulatedDevice, err := createEmulatedDevice(ctx, fv.BTPeers[0], s.Param().(cbt.DeviceType))
	if err != nil {
		s.Fatal("Failed to emulate the device type: ", err)
	}

	defer func() {
		if err = emulatedDevice.RPC().StopPairingAgent(cleanupCtx); err != nil {
			testing.ContextLog(cleanupCtx, "Failed to stop pairing agent on device")
		}
	}()

	qs := quicksettingsService.NewQuickSettingsServiceClient(fv.DUTRPCClient.Conn)
	uiautomation := ui.NewAutomationServiceClient(fv.DUTRPCClient.Conn)

	if _, err = qs.NavigateToBluetoothDetailedView(ctx, &emptypb.Empty{}); err != nil {
		s.Error("Failed to navigate to the detailed Bluetooth within Quick Settings: ", err)
	}

	if _, err := uiautomation.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: quicksettings.PairNewDeviceButton}); err != nil {
		s.Fatal("Failed to find the pair new device button: ", err)
	}
	if _, err := uiautomation.LeftClick(
		ctx, &ui.LeftClickRequest{Finder: quicksettings.PairNewDeviceButton}); err != nil {
		s.Fatal("Failed to click the pair new device button: ", err)
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
			{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_BUTTON}},
		},
	}

	kb := inputs.NewKeyboardServiceClient(fv.DUTRPCClient.Conn)

	defer func() {
		if res, err := uiautomation.IsNodeFound(cleanupCtx, &ui.IsNodeFoundRequest{
			Finder: quicksettings.PairNewDeviceDialog}); err != nil {
			testing.ContextLog(cleanupCtx, "Failed to determine if the pairing dialog was still open")
		} else if res.Found {
			if _, err := kb.Accel(cleanupCtx, &inputs.AccelRequest{
				Key: "Esc",
			}); err != nil {
				testing.ContextLog(cleanupCtx, "Failed to press escape to close the pairing dialog after failing to find the device")
			}
		}
	}()

	if _, err := uiautomation.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: emulatedDeviceFinder}); err != nil {
		uichrome := ui.NewChromeUIServiceClient(fv.DUTRPCClient.Conn)
		if _, err2 := uichrome.DumpUITree(ctx, &emptypb.Empty{}); err2 != nil {
			s.Fatal("Failed to dump the UI tree")
		}
		s.Fatal("Failed to find the emulated device in the pairing dialog with name "+string(emulatedDeviceName)+": ", err)
	}
	if _, err := uiautomation.LeftClick(
		ctx, &ui.LeftClickRequest{Finder: emulatedDeviceFinder}); err != nil {
		s.Fatal("Failed to click the emulated device in the pairing dialog: ", err)
	}

	if emulatedDevice.HasPinCode() {
		if _, err := kb.Type(ctx, &inputs.TypeRequest{
			Key: emulatedDevice.PinCode(),
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
