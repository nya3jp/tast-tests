// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cellular provides functions for testing Cellular connectivity.
package cellular

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const defaultTimeout = shillconst.DefaultTimeout

// Helper fetches Cellular Device and Service properties.
type Helper struct {
	Manager *shill.Manager
	Device  *shill.Device
}

// NewHelper creates a Helper object and ensures that a Cellular Device is present.
func NewHelper(ctx context.Context) (*Helper, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	available, err := manager.IsAvailable(ctx, shill.TechnologyCellular)
	if err != nil || !available {
		return nil, errors.Wrap(err, "Cellular Technology not available")
	}
	device, err := manager.DeviceByType(ctx, shillconst.TypeCellular)
	if err != nil || device == nil {
		return nil, errors.Wrap(err, "failed to get Cellular Device")
	}
	helper := Helper{Manager: manager, Device: device}

	// Ensure Cellular is enabled.
	if enabled, err := manager.IsEnabled(ctx, shill.TechnologyCellular); err != nil {
		return nil, errors.Wrap(err, "error requesting enabled state")
	} else if !enabled {
		if err := helper.Enable(ctx); err != nil {
			return nil, errors.Wrap(err, "unable to enable Cellular")
		}
	}
	// Disable pin lock with default pin and puk with dut puk if locked
	if err := helper.ClearSIMLock(ctx, mmconst.DefaultSimPin, ""); err != nil {
		return nil, errors.Wrap(err, "failed to unlock dut with default pin")
	}
	if err := helper.CaptureDBusLogs(ctx); err != nil {
		return nil, errors.Wrap(err, "unable to start DBus log capture")
	}
	return &helper, nil
}

// WaitForEnabledState polls for the specified enable state for cellular.
func (h *Helper) WaitForEnabledState(ctx context.Context, expected bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		enabled, err := h.Manager.IsEnabled(ctx, shill.TechnologyCellular)
		if err != nil {
			return errors.Wrap(err, "failed to get enabled state")
		}
		if enabled != expected {
			return errors.Errorf("unexpected enabled state, got %t, expected %t", enabled, expected)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  defaultTimeout,
		Interval: 500 * time.Millisecond,
	})
}

// Enable calls Manager.EnableTechnology(cellular) and returns true if the enable succeeded, or an error otherwise.
func (h *Helper) Enable(ctx context.Context) error {
	h.Manager.EnableTechnology(ctx, shill.TechnologyCellular)

	if err := h.WaitForEnabledState(ctx, true); err != nil {
		return err
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyPowered, true, defaultTimeout); err != nil {
		return errors.Wrap(err, "expected powered to become true, got false")
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyScanning, false, defaultTimeout); err != nil {
		return errors.Wrap(err, "expected scanning to become false, got true")
	}
	return nil
}

// Disable calls Manager.DisableTechnology(cellular) and returns true if the disable succeeded, or an error otherwise.
func (h *Helper) Disable(ctx context.Context) error {
	h.Manager.DisableTechnology(ctx, shill.TechnologyCellular)

	if err := h.WaitForEnabledState(ctx, false); err != nil {
		return err
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyPowered, false, defaultTimeout); err != nil {
		return err
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyScanning, false, defaultTimeout); err != nil {
		return errors.Wrap(err, "expected scanning to become false, got true")
	}
	return nil
}

// FindService returns the first connectable Cellular Service.
// If no such Cellular Service is available, returns a nil service and an error.
func (h *Helper) FindService(ctx context.Context) (*shill.Service, error) {
	// Look for any connectable Cellular service.
	cellularProperties := map[string]interface{}{
		shillconst.ServicePropertyConnectable: true,
		shillconst.ServicePropertyType:        shillconst.TypeCellular,
	}
	return h.Manager.WaitForServiceProperties(ctx, cellularProperties, defaultTimeout)
}

// FindServiceForDeviceWithTimeout returns the first connectable Cellular Service matching the Device ICCID.
// If no such Cellular Service is available, returns a nil service and an error.
// |timeout| specifies how long to wait for a service to appear.
func (h *Helper) FindServiceForDeviceWithTimeout(ctx context.Context, timeout time.Duration) (*shill.Service, error) {
	deviceProperties, err := h.Device.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Cellular Device properties")
	}
	deviceICCID, err := deviceProperties.GetString(shillconst.DevicePropertyCellularICCID)
	if err != nil {
		return nil, errors.Wrap(err, "device missing ICCID")
	}
	if deviceICCID == "" {
		return nil, errors.Wrap(err, "device has empty ICCID")
	}
	props := map[string]interface{}{
		shillconst.ServicePropertyCellularICCID: deviceICCID,
		shillconst.ServicePropertyConnectable:   true,
		shillconst.ServicePropertyType:          shillconst.TypeCellular,
	}
	service, err := h.Manager.WaitForServiceProperties(ctx, props, timeout)
	if err != nil {
		return nil, errors.Wrapf(err, "Service not found for: %+v", props)
	}
	return service, nil
}

// FindServiceForDevice returns the first connectable Cellular Service matching the Device ICCID.
// If no such Cellular Service is available, returns a nil service and an error.
// The default timeout is used for waiting for the service to appear.
func (h *Helper) FindServiceForDevice(ctx context.Context) (*shill.Service, error) {
	return h.FindServiceForDeviceWithTimeout(ctx, defaultTimeout)
}

// AutoConnectCleanupTime provides enough time for a successful dbus operation.
// If a timeout occurs during cleanup, the operation will fail anyway.
const AutoConnectCleanupTime = 1 * time.Second

// SetServiceAutoConnect sets the AutoConnect property of the Cellular Service
// associated with the Cellular Device to |autoConnect| if the current value
// does not match |autoConnect|.
// Returns true when Service.AutoConnect is set and the operation succeeds.
// Returns an error if any operation fails.
func (h *Helper) SetServiceAutoConnect(ctx context.Context, autoConnect bool) (bool, error) {
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get Cellular Service")
	}
	properties, err := service.GetProperties(ctx)
	if err != nil {
		return false, errors.Wrap(err, "unable to get properties")
	}
	curAutoConnect, err := properties.GetBool(shillconst.ServicePropertyAutoConnect)
	if err != nil {
		return false, errors.Wrap(err, "unable to get AutoConnect")
	}
	if autoConnect == curAutoConnect {
		return false, nil
	}
	if err := service.SetProperty(ctx, shillconst.ServicePropertyAutoConnect, autoConnect); err != nil {
		return false, errors.Wrap(err, "failed to set Service.AutoConnect")
	}
	return true, nil
}

// ConnectToDefault connects to the default Cellular Service.
func (h *Helper) ConnectToDefault(ctx context.Context) error {
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return err
	}
	return h.ConnectToService(ctx, service)
}

// ConnectToServiceWithTimeout connects to a Cellular Service with a specified timeout.
// It ensures that the connect attempt succeeds, repeating attempts if necessary.
// Otherwise an error is returned.
func (h *Helper) ConnectToServiceWithTimeout(ctx context.Context, service *shill.Service, timeout time.Duration) error {
	props, err := service.GetProperties(ctx)
	if err != nil {
		return err
	}
	name, err := props.GetString(shillconst.ServicePropertyName)
	if err != nil {
		return err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := service.Connect(ctx); err != nil {
			return err
		}
		if err := service.WaitForConnectedOrError(ctx); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  timeout,
		Interval: 5 * time.Second,
	}); err != nil {
		return errors.Wrapf(err, "connect to %s failed", name)
	}
	return nil
}

// ConnectToService connects to a Cellular Service.
// It ensures that the connect attempt succeeds, repeating attempts if necessary.
// Otherwise an error is returned.
func (h *Helper) ConnectToService(ctx context.Context, service *shill.Service) error {
	// Connect requires a longer default timeout than other operations.
	return h.ConnectToServiceWithTimeout(ctx, service, defaultTimeout*6)
}

// Disconnect from the Cellular Service and ensure that the disconnect succeeded, otherwise return an error.
func (h *Helper) Disconnect(ctx context.Context) error {
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return err
	}
	if err := service.Disconnect(ctx); err != nil {
		return err
	}
	return service.WaitForProperty(ctx, shillconst.ServicePropertyIsConnected, false, defaultTimeout)
}

// SetDeviceProperty sets a Device property and waits for the property to be set.
func (h *Helper) SetDeviceProperty(ctx context.Context, prop string, value interface{}, timeout time.Duration) error {
	pw, err := h.Device.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	// If the Modem is starting, SetProperty may fail, so poll while that is the case.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := h.Device.SetProperty(ctx, prop, value); err != nil {
			if strings.Contains(err.Error(), shillconst.ErrorModemNotStarted) {
				return err
			}
			return testing.PollBreak(err)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrapf(err, "unable to set Device property: %s", prop)
	}

	expectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := pw.Expect(expectCtx, prop, value); err != nil {
		return errors.Wrapf(err, "%s not set", prop)
	}
	return nil
}

// InitDeviceProperty sets a device property and returns a function to restore the initial value.
func (h *Helper) InitDeviceProperty(ctx context.Context, prop string, value interface{}) (func(ctx context.Context), error) {
	return initProperty(ctx, h.Device.PropertyHolder, prop, value)
}

// InitServiceProperty sets a service property and returns a function to restore the initial value.
func (h *Helper) InitServiceProperty(ctx context.Context, prop string, value interface{}) (func(ctx context.Context), error) {
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cellular service")
	}
	return initProperty(ctx, service.PropertyHolder, prop, value)
}

// PropertyCleanupTime provides enough time for a successful dbus operation at the end of the test.
const PropertyCleanupTime = 1 * time.Second

// initProperty sets a property and returns a function to restore the initial value.
func initProperty(ctx context.Context, properties *shill.PropertyHolder, prop string, value interface{}) (func(ctx context.Context), error) {

	prevValue, err := properties.GetAndSetProperty(ctx, prop, value)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read and initialize property")
	}

	return func(ctx context.Context) {
		if err := properties.SetProperty(ctx, prop, prevValue); err != nil {
			testing.ContextLogf(ctx, "Failed to restore %s: %s", prop, err)
		}
	}, nil

}

// RestartModemManager  - restart modemmanager with debug logs enabled/disabled.
// Return nil if restart succeeds, else return error.
func (h *Helper) RestartModemManager(ctx context.Context, enableDebugLogs bool) error {
	logLevel := "INFO"
	if enableDebugLogs {
		logLevel = "DEBUG"
	}

	if err := upstart.RestartJob(ctx, "modemmanager", upstart.WithArg("MM_LOGLEVEL", logLevel)); err != nil {
		return errors.Wrap(err, "failed to restart modemmanager")
	}

	return nil
}

// CaptureDBusLogs - Capture DBus system logs
// Return nil if DBus log collection succeeds, else return error.
func (h *Helper) CaptureDBusLogs(ctx context.Context) error {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get out dir")
	}

	f, err := os.Create(filepath.Join(outDir, "dbus-system.txt"))
	if err != nil {
		return err
	}

	cmd := testexec.CommandContext(ctx, "/usr/bin/dbus-monitor", "--system", "sender=org.chromium.flimflam", "destination=org.chromium.flimflam", "path_namespace=/org/freedesktop/ModemManager1")
	if f != nil {
		cmd.Stdout = f
		cmd.Stderr = f
	}

	if err := cmd.Start(); err != nil {
		f.Close()
		return errors.Wrap(err, "failed to start command")
	}

	go func() {
		defer cmd.Kill()
		defer cmd.Wait()
		defer f.Close()
		<-ctx.Done()
	}()

	return nil
}

// TODO: Errors needs to be handles across as per cbb
// Add here SIM pin functions
// Device function names.
// Possible Errors: [service].Error.InvalidArguments
// [service].Error.NotSupported
// [service].Error.PinError

// In the case of PinError, the error message gives
// more detail: [interface].PinRequired
// [interface].PinBlocked
// [interface].IncorrectPin

// IsSimLockEnabled returns lockenabled value
func (h *Helper) IsSimLockEnabled(ctx context.Context) bool {
	lockStatus, _ := h.GetCellularSIMLockStatus(ctx)
	lockEnabled := lockStatus[shillconst.DevicePropertyCellularSIMLockStatusLockEnabled].Value()
	return lockEnabled
}

// IsSimPinLocked returns true if locktype value is sim-pin
func (h *Helper) IsSimPinLocked(ctx context.Context) bool {
	lockStatus, _ := h.GetCellularSIMLockStatus(ctx)
	lockType := lockStatus[shillconst.DevicePropertyCellularSIMLockStatusLockType].Value()
	return lockType == shillconst.DevicePropertyValueSIMLockTypePIN
}

// IsSimPukLocked returns true if locktype value is sim-puk
func (h *Helper) IsSimPukLocked(ctx context.Context) bool {
	lockStatus, _ := h.GetCellularSIMLockStatus(ctx)
	lockType := lockStatus[shillconst.DevicePropertyCellularSIMLockStatusLockType].Value()
	return lockType == shillconst.DevicePropertyValueSIMLockTypePUK
}

// GetRetriesLeft helps to get modem property UnlockRetries value
func (h *Helper) GetRetriesLeft(ctx context.Context) (int, error) {
	lockStatus, _ := h.GetCellularSIMLockStatus(ctx)
	retriesLeft := lockStatus[shillconst.DevicePropertyCellularSIMLockStatusRetriesLeft].Value()
	if retriesLeft == nil {
		return errors.Wrap("failed to get RetriesLeft")
	}
	if retriesLeft == none {
		return 0, errors.Wrap("missing RetriesLeft property")
	}
	if retriesLeft < 0 {
		return 0, errors.Wrap("malformed RetriesLeft: %d", retriesLeft)
	}
	return retriesLeft, nil
}

// UnlockDut is to pin unlock before every test
func (h *Helper) UnlockDut(ctx context.Context, currentPin, currentPuk string) error {
	// Check if PIN enabled and locked/set
	if h.IsSimLockEnabled(ctx) && h.IsSimPinLocked(ctx) {
		// Disable and remove PIN
		err := h.Device.RequirePin(currentPin, false)
		if err != nil {
			return 0, errors.Wrap(err, "failed to disable lock")
		}
	}
	return nil
}

// ClearSIMLock clears puk, pin lock if lockenabled
func (h *Helper) ClearSIMLock(ctx context.Context, pin, puk string) error {

	if h.IsSimLockEnabled(ctx) {
		// clear puk lock
		if h.IsSimPukLocked(ctx) {
			err := h.Device.UnblockPUK(puk, pin)
			if err != nil {
				return errors.Wrap(err, "failed to UnblockPUK")
			}
		}
		// clear pin lock
		if h.IsSimPinLocked(ctx) {
			err := h.Device.EnterPin(pin)
			if err == shillconst.ErrorIncorrectPin {
				// Do max unlock tries and do puk unlock
				err = h.PukLockSim(ctx, pin)
				if err != nil {
					return errors.Wrap(err, "failed to PukLockSim with pin in ClearSIMLock")
				}
				err = h.Device.UnblockPUK(puk, pin)
				if err != nil {
					return errors.Wrap(err, "failed to clear with UnblockPUK")
				}
				err = h.Device.EnterPin(pin)
				if err != nil {
					return errors.Wrap(err, "failed to clear pin lock with EnterPin")
				}
			}
		}
		// disable sim lock
		err := h.Device.RequirePin(ctx, pin, false)
		if err != nil {
			return errors.Wrap(err, "failed to clear pin lock with RequirePin")
		}
	}
	return nil
}

// GetCellularSIMLockStatus dict gets Cellular.SIMLockStatus dictionary
func (h *Helper) GetCellularSIMLockStatus(ctx context.Context) ([]map[string]dbus.Variant, error) {
	// Gather Shill Device properties
	deviceProps, err := h.Device.GetShillProperties(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get device properties")
	}

	// Verify Device.SimSlots.
	info, err := deviceProps.Get(shillconst.DevicePropertyCellularSIMLockStatus)
	if err != nil {
		return errors.Wrap(err, "failed to get device cellularsimlockstatus property")
	}
	simLockStatus, ok := info.([]map[string]dbus.Variant)
	if !ok {
		return errors.Wrap(err, "invalid format for device cellularsimlockstatus")
	}
	return simLockStatus, nil
}

// Helper functions for SIM lock/unlock

// random generates a random integer
func random(min, max int) int {
	return rand.Intn(max-min) + min
}

// BadPin obtains a pin that does not match the valid sim-pin.
func (h *Helper) BadPin(ctx context.Context, currentPin string) (string, error) {
	randomPin := random(1000, 9999)
	pin, _ := strconv.Atoi(currentPin)
	if randomPin == pin {
		randomPin++
	}
	return strconv.Itoa(pin), nil
}

// BadPuk obtains a puk that does not match the valid sim-puk.
func (h *Helper) BadPuk(ctx context.Context, currentPuk string) (string, error) {
	randomPuk := random(10000000, 99999999)
	puk, _ := strconv.Atoi(currentPuk)
	if randomPuk == puk {
		randomPuk++
	}
	return strconv.Itoa(puk), nil
}

// PinLockSim is a helper method to pin-lock a SIM, assuming nothing bad happens.
func (h *Helper) PinLockSim(ctx context.Context, newPin string) error {
	if err := h.Device.RequirePin(newPin, true); err != nil {
		return errors.Wrap(err, "failed to enable with new pin")
	}
	return nil
}

// PukLockSim is a helper method to puk-lock a SIM, assuming nothing bad happens.
func (h *Helper) PukLockSim(ctx context.Context, currentPin string) error {
	if err := h.PinLockSim(ctx, currentPin); err != nil {
		return errors.Wrap(err, "failed at puklocksim")
	}
	locked := false
	for !locked {
		locked := h.IsSimPukLocked(ctx)
		if locked == true {
			break
		}
		err := h.EnterIncorrectPin(ctx, currentPin)
		if err.Error() == "PIN Blocked Error" {
			return errors.Wrap(err, "sim could not get blocked")
		}
	}
	if !h.IsSimPukLocked(ctx) {
		return errors.Wrap("expected sim to be puk-locked")
	}
}

// EnterIncorrectPin checks expected error for bad pin given
func (h *Helper) EnterIncorrectPin(ctx context.Context, currentPin string) error {
	badPin, err := h.BadPin(ctx, currentPin)
	if err != nil {
		return errors.Wrap(err, "failed to generate bad pin")
	}
	if err = h.Device.EnterPin(badPin); err == nil {
		return errors.Wrap(err, "failed to send bad pin")
	}
	// errorIncorrectPin used to do graceful exit for expected bad pin error
	// TODO: ERROR_INCORRECT_PIN = 'org.freedesktop.ModemManager1.Sim.Error.IncorrectPin'
	errorIncorrectPin := errors.New("org.freedesktop.ModemManager1.Sim.Error.IncorrectPin")

	if err == errorIncorrectPin {
		return nil
	}
	return errors.Wrap(err, "unusual pin error")
}

// EnterIncorrectPuk checks expected error for bad puk given
func (h *Helper) EnterIncorrectPuk(ctx context.Context, currentPuk string) error {
	badPuk, err := h.BadPuk(ctx, currentPuk)
	if err != nil {
		return errors.Wrap(err, "failed to generate bad puk")
	}
	if err = h.Device.UnblockPUK(badPuk); err == nil {
		return errors.Wrap(err, "failed to send bad puk")
	}
	// errorIncorrectPuk used to do graceful exit for expected bad puk error
	// TODO: ERROR_INCORRECT_PUK = 'org.freedesktop.ModemManager1.Sim.Error.IncorrectPuk'
	errorIncorrectPuk := errors.New("org.freedesktop.ModemManager1.Sim.Error.IncorrectPuk")
	if err == errorIncorrectPuk {
		return nil
	}
	return errors.Wrap(err, "unusual puk error")
}
