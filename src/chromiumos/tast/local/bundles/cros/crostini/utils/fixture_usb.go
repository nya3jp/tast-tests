package utils

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type UsbController struct {
	systemCount int
}

// to create object to control usb fixture
// also record system usb count (as condition: plug in station without any usb)
func NewUsbController(ctx context.Context, s *testing.State) (*UsbController, error) {

	s.Log("Starting create usb recorder")

	// plug in station
	if err := ControlFixture(ctx, s, FixtureStation, ActionPlugin, false); err != nil {
		return nil, err
	}

	// get system + station total usb count
	count, err := GetUsbCount(ctx, s)
	if err != nil {
		return nil, err
	}

	// unplug station
	if err := ControlFixture(ctx, s, FixtureStation, ActionUnplug, false); err != nil {
		return nil, err
	}

	s.Log("Usb recorder created")

	return &UsbController{
		systemCount: count,
	}, nil
}

// execute command 'lsusb' to list usb info
// get length of array
func GetUsbCount(ctx context.Context, s *testing.State) (int, error) {

	var array []string

	// use command to list usb devices
	lsusb := testexec.CommandContext(ctx, "lsusb")
	out, err := lsusb.Output()
	if err != nil {
		return -1, err
	} else {

		// split string
		result := strings.TrimSpace(string(out))
		devices := strings.Split(result, "\n")

		// append to device array
		for _, device := range devices {
			if strings.Contains(strings.ToLower(device), "device") {
				array = append(array, device)
			}
		}
	}

	return len(array), nil
}

// according to input state
// verify current usb count is correct to input count
func (ur *UsbController) VerifyUsbCount(ctx context.Context, s *testing.State, state ConnectState) error {

	inputCount, err := GetInputArgumentCount(ctx, s, PeripheralUsb)
	if err != nil {
		return err
	}

	// get current usb count
	currentCount, err := GetUsbCount(ctx, s)
	if err != nil {
		return err
	}

	// verify usb's count
	if state { // usb connected
		difference := currentCount - ur.systemCount
		if difference != inputCount {
			return errors.Errorf("failed to verify connected usb, system is %d, current is %d:, input is %d ", ur.systemCount, currentCount, inputCount)
		}
	} else { // usb disconnect
		// 1. usb & station disconnect
		// 2. usb disconnect and station connect
		if currentCount > ur.systemCount {
			return errors.Errorf("failed to verify usb when disconnected: system is %d, current is %d", ur.systemCount, currentCount)
		}

	}

	return nil
}

// according to input argument
// control usbs to plug in / unplug, one by one
func (ur *UsbController) ControlUsbs(ctx context.Context, s *testing.State, action ActionState, needToDelay bool) error {

	// input argument array
	args, err := GetInputArgument(ctx, s, PeripheralUsb)
	if err != nil {
		return err
	}

	// control fixture by input argument array
	if err := ControlFixtureByArgument(ctx, s, args, action, needToDelay); err != nil {
		return err
	}

	return nil
}
