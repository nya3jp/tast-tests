// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cellular provides functions for testing Cellular connectivity.
package cellular

import (
	"context"
	"io/ioutil"
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
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const defaultTimeout = shillconst.DefaultTimeout

// Helper fetches Cellular Device and Service properties.
type Helper struct {
	Manager *shill.Manager
	Device  *shill.Device
}

// NewHelper creates a Helper object and ensures that a Cellular Device is present.
func NewHelper(ctx context.Context) (*Helper, error) {
	ctx, st := timing.Start(ctx, "Helper.NewHelper")
	defer st.End()

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
		if _, err := helper.Enable(ctx); err != nil {
			return nil, errors.Wrap(err, "unable to enable Cellular")
		}
	}
	// Disable pin lock with default pin and puk with dut puk if locked.
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
func (h *Helper) Enable(ctx context.Context) (time.Duration, error) {
	ctx, st := timing.Start(ctx, "Helper.Enable")
	defer st.End()

	start := time.Now()
	h.Manager.EnableTechnology(ctx, shill.TechnologyCellular)

	if err := h.WaitForEnabledState(ctx, true); err != nil {
		return 0, err
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyPowered, true, defaultTimeout); err != nil {
		return 0, errors.Wrap(err, "expected powered to become true, got false")
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyScanning, false, defaultTimeout); err != nil {
		return 0, errors.Wrap(err, "expected scanning to become false, got true")
	}
	return time.Since(start), nil
}

// Disable calls Manager.DisableTechnology(cellular) and returns true if the disable succeeded, or an error otherwise.
func (h *Helper) Disable(ctx context.Context) (time.Duration, error) {
	ctx, st := timing.Start(ctx, "Helper.Disable")
	defer st.End()

	start := time.Now()
	h.Manager.DisableTechnology(ctx, shill.TechnologyCellular)

	if err := h.WaitForEnabledState(ctx, false); err != nil {
		return 0, err
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyPowered, false, defaultTimeout); err != nil {
		return 0, err
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyScanning, false, defaultTimeout); err != nil {
		return 0, errors.Wrap(err, "expected scanning to become false, got true")
	}
	return time.Since(start), nil
}

// FindService returns the first connectable Cellular Service.
// If no such Cellular Service is available, returns a nil service and an error.
func (h *Helper) FindService(ctx context.Context) (*shill.Service, error) {
	ctx, st := timing.Start(ctx, "Helper.FindService")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "Helper.FindServiceForDeviceWithTimeout")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "Helper.FindServiceForDevice")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "Helper.SetServiceAutoConnect")
	defer st.End()

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
func (h *Helper) ConnectToDefault(ctx context.Context) (time.Duration, error) {
	ctx, st := timing.Start(ctx, "Helper.ConnectToDefault")
	defer st.End()

	start := time.Now()
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return 0, err
	}
	if err := h.ConnectToService(ctx, service); err != nil {
		return 0, err
	}

	return time.Since(start), nil
}

// ConnectToServiceWithTimeout connects to a Cellular Service with a specified timeout.
// It ensures that the connect attempt succeeds, repeating attempts if necessary.
// Otherwise an error is returned.
func (h *Helper) ConnectToServiceWithTimeout(ctx context.Context, service *shill.Service, timeout time.Duration) error {
	ctx, st := timing.Start(ctx, "Helper.ConnectToServiceWithTimeout")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "Helper.ConnectToService")
	defer st.End()

	// Connect requires a longer default timeout than other operations.
	return h.ConnectToServiceWithTimeout(ctx, service, defaultTimeout*6)
}

// Disconnect from the Cellular Service and ensure that the disconnect succeeded, otherwise return an error.
func (h *Helper) Disconnect(ctx context.Context) (time.Duration, error) {
	ctx, st := timing.Start(ctx, "Helper.Disconnect")
	defer st.End()

	start := time.Now()
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return 0, err
	}
	if err := service.Disconnect(ctx); err != nil {
		return 0, err
	}
	if err := service.WaitForProperty(ctx, shillconst.ServicePropertyIsConnected, false, defaultTimeout); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

// SetDeviceProperty sets a Device property and waits for the property to be set.
func (h *Helper) SetDeviceProperty(ctx context.Context, prop string, value interface{}, timeout time.Duration) error {
	ctx, st := timing.Start(ctx, "Helper.SetDeviceProperty")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "Helper.InitDeviceProperty")
	defer st.End()

	return initProperty(ctx, h.Device.PropertyHolder, prop, value)
}

// InitServiceProperty sets a service property and returns a function to restore the initial value.
func (h *Helper) InitServiceProperty(ctx context.Context, prop string, value interface{}) (func(ctx context.Context), error) {
	ctx, st := timing.Start(ctx, "Helper.InitServiceProperty")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "Helper.initProperty")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "Helper.RestartModemManager")
	defer st.End()

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

// ResetModem calls Device.ResetModem(cellular) and returns true if the reset succeeded, or an error otherwise.
func (h *Helper) ResetModem(ctx context.Context) (time.Duration, error) {
	ctx, st := timing.Start(ctx, "Helper.Enable")
	defer st.End()

	start := time.Now()

	if err := h.Device.Reset(ctx); err != nil {
		return time.Since(start), errors.Wrap(err, "reset modem failed")
	}

	testing.ContextLog(ctx, "Reset modem called")

	if err := h.WaitForEnabledState(ctx, true); err != nil {
		return time.Since(start), err
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyPowered, true, defaultTimeout); err != nil {
		return time.Since(start), errors.Wrap(err, "expected powered to become true, got false")
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyScanning, false, defaultTimeout); err != nil {
		return time.Since(start), errors.Wrap(err, "expected scanning to become false, got true")
	}
	// Sleep added as reset fibocmm modem taking time for modem state to
	// registered and service to refresh. Not found any state/property to wait.
	// TODO(b/216176362) : Reset modem causes pin api calls failure if not waited 30 seconds.
	if err := testing.Sleep(ctx, 30*time.Second); err != nil {
		return time.Since(start), errors.Wrap(err, "failed to sleep after reset modem")
	}

	return time.Since(start), nil
}

// IsSimLockEnabled returns lockenabled value.
func (h *Helper) IsSimLockEnabled(ctx context.Context) bool {
	lockStatus, _ := h.GetCellularSIMLockStatus(ctx)
	lockEnabled := lockStatus[shillconst.DevicePropertyCellularSIMLockStatusLockEnabled]
	testing.ContextLog(ctx, "lock enabled status: ", lockEnabled)
	return lockEnabled.(bool)
}

// IsSimPinLocked returns true if locktype value is 'sim-pin'
// locktype value is 'sim-pin2' for QC and 'none' when not locked.
func (h *Helper) IsSimPinLocked(ctx context.Context) bool {
	lockStatus, err := h.GetCellularSIMLockStatus(ctx)
	if err != nil {
		testing.ContextLog(ctx, "getcellularsimlockstatus -pin: ", err.Error())
	}

	lockType := lockStatus[shillconst.DevicePropertyCellularSIMLockStatusLockType]
	lock := ""
	if lockType != nil {
		lock = lockType.(string)
		testing.ContextLog(ctx, "pin lock type value: ", lock)
	}

	return lock == shillconst.DevicePropertyValueSIMLockTypePIN
}

// IsSimPukLocked returns true if locktype value is 'sim-puk'
// locktype value is 'sim-pin2' for QC and value 'none' when not locked.
func (h *Helper) IsSimPukLocked(ctx context.Context) bool {
	lockStatus, err := h.GetCellularSIMLockStatus(ctx)
	if err != nil {
		testing.ContextLog(ctx, "getcellularsimlockstatus -puk: ", err.Error())
	}

	lockType := lockStatus[shillconst.DevicePropertyCellularSIMLockStatusLockType]
	lock := ""
	if lockType != nil {
		lock = lockType.(string)
		testing.ContextLog(ctx, "puk locktype: ", lock)
	}

	return lock == shillconst.DevicePropertyValueSIMLockTypePUK
}

// GetRetriesLeft helps to get modem property UnlockRetries value.
func (h *Helper) GetRetriesLeft(ctx context.Context) (int32, error) {
	lockStatus, err := h.GetCellularSIMLockStatus(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "getcellularsimlockstatus failed reading retriesleft")
	}

	retriesLeft := lockStatus[shillconst.DevicePropertyCellularSIMLockStatusRetriesLeft]

	if retriesLeft == nil {
		return 0, errors.New("failed to get retriesLeft")
	}
	retries, ok := retriesLeft.(int32)
	if !ok {
		return 0, errors.New("int32 type assert failed for retriesleft property")
	}
	if retries < 0 {
		return 0, errors.New("negative retriesleft property")
	}
	testing.ContextLog(ctx, "retriesleft value : ", retries)

	return retries, nil
}

// In the case of [service].Error.PinError, the error message gives details:
// [interface].PinRequired
// [interface].PinBlocked
// [interface].IncorrectPin

// UnlockDut is to unlock sim pin before every test.
func (h *Helper) UnlockDut(ctx context.Context, currentPin, currentPuk string) error {
	// Check if pin enabled and locked/set.
	if h.IsSimLockEnabled(ctx) || h.IsSimPinLocked(ctx) {
		// Disable pin.
		if err := h.Device.RequirePin(ctx, currentPin, false); err != nil {
			return errors.Wrap(err, "failed to disable lock")
		}
	}

	return nil
}

// ClearSIMLock clears puk, pin lock if any of them enabled and locked.
func (h *Helper) ClearSIMLock(ctx context.Context, pin, puk string) error {

	if !h.IsSimLockEnabled(ctx) && !h.IsSimPukLocked(ctx) {
		return nil
	}

	// Clear puk lock if puk locked which is unusual.
	if h.IsSimPukLocked(ctx) {
		if len(puk) == 0 {
			modem, err := modemmanager.NewModemWithSim(ctx)
			if err != nil {
				return errors.Wrap(err, "could not find mm dbus object with a valid sim")
			}
			puk, err = modem.GetActiveSimPuk(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get active sim puk in clearsimlock")
			}
		}
		if err := h.Device.UnblockPin(ctx, puk, pin); err != nil {
			return errors.Wrap(err, "failed to UnblockPin")
		}
	}

	// Clear pin lock, this can also happen after puk unlocked.
	if h.IsSimPinLocked(ctx) {
		errStr := ""
		if err := h.Device.EnterPin(ctx, pin); err != nil {
			errStr = err.Error()
			testing.ContextLog(ctx, "at enterpin in clearsimlock "+errStr)
		}
		if errStr != "" && !strings.Contains(errStr, shillconst.ErrorIncorrectPin) {
			// Do max unlock tries and do puk unlock.
			if err := h.PukLockSim(ctx, pin); err != nil {
				return errors.Wrap(err, "failed to PukLockSim with pin in ClearSIMLock")
			}
			if err := h.Device.UnblockPin(ctx, puk, pin); err != nil {
				return errors.Wrap(err, "failed to clear with UnblockPin")
			}
			if err := h.Device.EnterPin(ctx, pin); err != nil {
				return errors.Wrap(err, "failed to clear pin lock with EnterPin")
			}
		}
	}

	// Disable sim lock.
	if err := h.Device.RequirePin(ctx, pin, false); err != nil {
		return errors.Wrap(err, "failed to clear pin lock with RequirePin")
	}

	testing.ContextLog(ctx, "clearsimlock disabled pin is: ", pin)
	h.ResetModem(ctx)
	testing.ContextLog(ctx, "reset modem after clearing sim pin or puk lock")

	return nil
}

// GetCellularSIMLockStatus dict gets Cellular.SIMLockStatus dictionary from shill properties.
func (h *Helper) GetCellularSIMLockStatus(ctx context.Context) (map[string]interface{}, error) {
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyScanning, false, defaultTimeout); err != nil {
		return nil, errors.Wrap(err, "expected scanning to become false, got true")
	}
	// Gather Shill Device properties.
	deviceProps, err := h.Device.GetShillProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device properties")
	}

	// Verify Device.SimSlots.
	info, err := deviceProps.Get(shillconst.DevicePropertyCellularSIMLockStatus)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device cellularsimlockstatus property")
	}

	simLockStatus := make(map[string]interface{})
	simLockStatus, ok := info.(map[string]interface{})
	if !ok {
		return nil, errors.Wrap(err, "invalid format for device cellularsimlockstatus")
	}

	testing.ContextLog(ctx, "simlockstatus: ", simLockStatus)

	return simLockStatus, nil
}

// Helper functions for SIM lock/unlock.

// random generates a random integer in given range.
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

	return strconv.Itoa(randomPin), nil
}

// BadPuk obtains a puk that does not match the valid sim-puk.
func (h *Helper) BadPuk(ctx context.Context, currentPuk string) (string, error) {
	randomPuk := random(10000000, 99999999)
	puk, _ := strconv.Atoi(currentPuk)
	if randomPuk == puk {
		randomPuk++
	}

	return strconv.Itoa(randomPuk), nil
}

// PinLockSim is a helper method to pin-lock a sim, assuming nothing bad happens.
func (h *Helper) PinLockSim(ctx context.Context, newPin string) error {
	if h.IsSimPinLocked(ctx) {
		return nil
	}
	if err := h.Device.RequirePin(ctx, newPin, true); err != nil {
		return errors.Wrap(err, "failed to enable lock with new pin")
	}

	return nil
}

// PukLockSim is a helper method to puk-lock a SIM, assuming nothing bad happens.
func (h *Helper) PukLockSim(ctx context.Context, currentPin string) error {
	if err := h.PinLockSim(ctx, currentPin); err != nil {
		return errors.Wrap(err, "failed at pinlocksim")
	}

	// Reset modem to reflect if puk locked.
	if _, err := h.ResetModem(ctx); err != nil {
		return errors.Wrap(err, "reset modem failed after pin lock set")
	}

	locked := h.IsSimPinLocked(ctx)
	if locked == true {
		testing.ContextLog(ctx, "pinlocked with: ", currentPin)
	}
	locked = false
	retriesCnt := 0
	// Max incorrect retries for pin to be puk locked are 3, change to constant.
	// TODO(b/216176362): Unexpected RetriesLeft value(999) after modem reset on fibocomm modem.
	for retriesCnt < 3 {
		if err := h.EnterIncorrectPin(ctx, currentPin); err != nil {
			return errors.Wrap(err, "incorrect pin entries failed")
		}
		if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyScanning, false, defaultTimeout); err != nil {
			return errors.Wrap(err, "expected scanning to become false, got true")
		}
		retriesCnt++
	}

	locked = h.IsSimPukLocked(ctx)
	if !locked {
		return errors.New("expected sim to be puk-locked")
	}

	return nil
}

// EnterIncorrectPin gets incorrect pin and tries to unlock.
func (h *Helper) EnterIncorrectPin(ctx context.Context, currentPin string) error {
	badPin, err := h.BadPin(ctx, currentPin)
	testing.ContextLog(ctx, "Created badpin is: ", badPin)
	if err != nil {
		return errors.Wrap(err, "failed to generate bad pin")
	}

	// TODO(b/216167098): Incorrect enterpin call returning unexpected error in 3rd try on Octopus.
	// Fails on fibocomm modems as not getting one of the expected error.
	if err = h.Device.EnterPin(ctx, badPin); err == nil {
		return errors.Wrap(err, "failed to send bad pin: "+badPin)
	}

	// For flimflam errors with org.chromium.flimflam.Error.IncorrectPin.
	if strings.Contains(err.Error(), shillconst.ErrorIncorrectPin) ||
		strings.Contains(err.Error(), shillconst.ErrorIncorrectPassword) ||
		strings.Contains(err.Error(), shillconst.ErrorPukRequired) ||
		strings.Contains(err.Error(), shillconst.ErrorPinBlocked) {
		return nil
	}

	return errors.Wrap(err, "unusual pin error")
}

// EnterIncorrectPuk generates bad puk and tries to unlock.
func (h *Helper) EnterIncorrectPuk(ctx context.Context, currentPuk string) error {
	badPuk, err := h.BadPuk(ctx, currentPuk)
	if err != nil {
		return errors.Wrap(err, "failed to generate bad puk")
	}

	if err = h.Device.UnblockPin(ctx, badPuk, mmconst.DefaultSimPin); err == nil {
		return errors.Wrap(err, "failed to send bad puk: "+badPuk)
	}

	errorIncorrectPuk := errors.New("org.freedesktop.ModemManager1.Sim.Error.IncorrectPuk")
	if errors.Is(err, errorIncorrectPuk) {
		return nil
	}

	return errors.Wrap(err, "unusual puk error")
}

// SetServiceProvidersOverride adds an override MODB to shill.
func SetServiceProvidersOverride(ctx context.Context, sourceFile string) error {
	// TODO: return error
	input, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read %q", sourceFile)

	}

	err = ioutil.WriteFile("/usr/share/shill/serviceproviders-override.pbf", input, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write %q", "/usr/share/shill/serviceproviders-override.pbf")
	}
	return nil
}

// getCellularServiceDictProperty dict gets a shill service dictionary property
func (h *Helper) getCellularServiceDictProperty(ctx context.Context, propertyName string) (map[string]string, error) {
	// Verify that a connectable Cellular service exists and ensure it is connected.
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to find Cellular Service for Device")
	}
	props, err := service.GetShillProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting Service properties")
	}
	info, err := props.Get(propertyName)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting property %q", propertyName)
	}

	// dictProp := make(map[string]string)
	dictProp, ok := info.(map[string]string)
	if !ok {
		return nil, errors.Wrapf(err, "invalid format for %q", propertyName)
	}

	return dictProp, nil
}

// GetCellularLastAttachAPN dict gets Cellular.LastAttachAPN dictionary from shill properties.
func (h *Helper) GetCellularLastAttachAPN(ctx context.Context) (map[string]string, error) {
	return h.getCellularServiceDictProperty(ctx, shillconst.ServicePropertyCellularLastAttachAPN)
}

// GetCellularLastGoodAPN dict gets Cellular.LastAttachAPN dictionary from shill properties.
func (h *Helper) GetCellularLastGoodAPN(ctx context.Context) (map[string]string, error) {
	return h.getCellularServiceDictProperty(ctx, shillconst.ServicePropertyCellularLastGoodAPN)
}
