// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cellular provides functions for testing Cellular connectivity.
package cellular

import (
	"context"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const defaultTimeout = shillconst.DefaultTimeout

// ModemInfo Gets modem info from host_info_labels.
type ModemInfo struct {
	Type           string
	IMEI           string
	SupportedBands string
	SimCount       int
}

// SIMProfileInfo Gets profile info from host_info_labels.
type SIMProfileInfo struct {
	ICCID       string
	SimPin      string
	SimPuk      string
	CarrierName string
}

// SIMInfo Gets SIM info from host_info_labels.
type SIMInfo struct {
	SlotID      int
	Type        string
	EID         string
	TestEsim    bool
	ProfileInfo []*SIMProfileInfo
}

// LabelMap is the type label map.
type LabelMap map[string][]string

// Helper fetches Cellular Device and Service properties.
type Helper struct {
	Manager            *shill.Manager
	Device             *shill.Device
	enableEthernetFunc func(ctx context.Context)
	enableWifiFunc     func(ctx context.Context)
	Labels             []string
	modemInfo          *ModemInfo
	simInfo            []*SIMInfo
	carrierName        string
	devicePools        []string
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
		testing.ContextLog(ctx, "Warning: Unable to start DBus log capture: ", err)
	}
	return &helper, nil
}

// NewHelperWithLabels creates a Helper object and populates label info from host_info_labels, ensuring that a Cellular Device is present.
func NewHelperWithLabels(ctx context.Context, labels []string) (*Helper, error) {
	helper, err := NewHelper(ctx)
	if err != nil {
		return nil, err
	}

	if err := helper.GetHostInfoLabels(ctx, labels); err != nil {
		return nil, errors.Wrap(err, "unable to read labels")
	}

	return helper, nil
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

// FindServiceForDeviceWithProps returns the first connectable Cellular Service matching the Device ICCID and the given props.
// If no such Cellular Service is available, returns a nil service and an error.
func (h *Helper) FindServiceForDeviceWithProps(ctx context.Context, props map[string]interface{}) (*shill.Service, error) {
	ctx, st := timing.Start(ctx, "Helper.FindServiceForDeviceWithProps")
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

	necessaryProps := map[string]interface{}{
		shillconst.ServicePropertyCellularICCID: deviceICCID,
		shillconst.ServicePropertyConnectable:   true,
		shillconst.ServicePropertyType:          shillconst.TypeCellular,
	}
	for k, v := range necessaryProps {
		props[k] = v
	}

	service, err := h.Manager.WaitForServiceProperties(ctx, props, defaultTimeout)
	if err != nil {
		return nil, errors.Wrapf(err, "Service not found for: %+v", props)
	}
	return service, nil
}

// FindServiceForDevice is a convenience function to call FindServiceForDeviceWithProps with an empty set of extra props.
func (h *Helper) FindServiceForDevice(ctx context.Context) (*shill.Service, error) {
	ctx, st := timing.Start(ctx, "Helper.FindServiceForDevice")
	defer st.End()

	return h.FindServiceForDeviceWithProps(ctx, make(map[string]interface{}))
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

// IsConnected return err if shillconst.ServicePropertyIsConnected is not true
func (h *Helper) IsConnected(ctx context.Context) error {
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return err
	}
	if err := service.WaitForProperty(ctx, shillconst.ServicePropertyIsConnected, true, defaultTimeout); err != nil {
		return err
	}
	return nil
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

// ResetShill restarts shill and clears all profiles.
func (h *Helper) ResetShill(ctx context.Context) []error {
	var errs []error
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to stop shill"))
	}
	if err := os.Remove(shillconst.DefaultProfilePath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, errors.Wrap(err, "failed to remove default profile"))
	}
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		// No more can be done if shill doesn't start
		return append(errs, errors.Wrap(err, "failed to restart shill"))
	}
	manager, err := shill.NewManager(ctx)
	if err != nil {
		// No more can be done if a manager interface cannot be created
		return append(errs, errors.Wrap(err, "failed to create new shill manager"))
	}
	// Disable Wifi to avoid log pollution
	manager.DisableTechnology(ctx, shill.TechnologyWifi)

	if err = manager.PopAllUserProfiles(ctx); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to pop all user profiles"))
	}

	// Wait until a service is connected.
	expectProps := map[string]interface{}{
		shillconst.ServicePropertyIsConnected: true,
	}
	if _, err := manager.WaitForServiceProperties(ctx, expectProps, 30*time.Second); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to wait for connected service"))
	}

	return errs
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
	ctx, st := timing.Start(ctx, "Helper.ResetModem")
	defer st.End()

	start := time.Now()

	if err := h.Device.Reset(ctx); err != nil {
		return time.Since(start), errors.Wrap(err, "reset modem failed")
	}

	testing.ContextLog(ctx, "Reset modem called")

	if err := h.WaitForEnabledState(ctx, false); err != nil {
		return time.Since(start), err
	}
	if err := h.WaitForEnabledState(ctx, true); err != nil {
		return time.Since(start), err
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyPowered, true, defaultTimeout); err != nil {
		return time.Since(start), errors.Wrap(err, "expected powered to become true, got false")
	}
	if err := h.Device.WaitForProperty(ctx, shillconst.DevicePropertyScanning, false, defaultTimeout); err != nil {
		return time.Since(start), errors.Wrap(err, "expected scanning to become false, got true")
	}

	return time.Since(start), nil
}

// IsSimLockEnabled returns lockenabled value.
func (h *Helper) IsSimLockEnabled(ctx context.Context) bool {
	lockStatus, err := h.GetCellularSIMLockStatus(ctx)
	if err != nil {
		testing.ContextLog(ctx, "getcellularsimlockstatus -lock: ", err.Error())
	}

	lock := false
	lockEnabled := lockStatus[shillconst.DevicePropertyCellularSIMLockStatusLockEnabled]
	if lockEnabled != nil {
		testing.ContextLog(ctx, "lock enabled status: ", lockEnabled)
		lock = lockEnabled.(bool)
	}
	return lock
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

	// If puk locked with unknown error, ignoring error as intention is to do puk lock.
	locked := h.IsSimPukLocked(ctx)
	if locked {
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

// getCellularDeviceDictProperty gets a shill device dictionary property
func (h *Helper) getCellularDeviceDictProperty(ctx context.Context, propertyName string) (map[string]string, error) {
	props, err := h.Device.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Device properties")
	}
	info, err := props.Get(propertyName)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting property %q", propertyName)
	}

	dictProp, ok := info.(map[string]string)
	if !ok {
		return nil, errors.Wrapf(err, "invalid format for %q", propertyName)
	}

	return dictProp, nil
}

// GetHomeProviderFromShill returns current home provider UUID and code from shill.
func (h *Helper) GetHomeProviderFromShill(ctx context.Context) (uuid, code string, err error) {
	ctx, st := timing.Start(ctx, "Helper.GetHomeProviderFromShill")
	defer st.End()
	homeProviderMap, err := h.getCellularDeviceDictProperty(ctx, shillconst.DevicePropertyCellularHomeProvider)
	if err != nil {
		return "", "", errors.New("invalid format for Home Provider property")
	}
	uuid, ok := homeProviderMap[shillconst.OperatorUUIDKey]
	if !ok {
		return "", "", errors.New("home provider UUID not found")
	}
	code, ok = homeProviderMap[shillconst.OperatorCode]
	if !ok {
		return "", "", errors.New("home provider operator code not found")
	}

	return uuid, code, nil
}

// GetServingOperatorFromShill returns current serving operator UUID and code from shill.
func (h *Helper) GetServingOperatorFromShill(ctx context.Context) (uuid, code string, err error) {
	ctx, st := timing.Start(ctx, "Helper.GetServingOperatorFromShill")
	defer st.End()
	servingOperatorMap, err := h.getCellularServiceDictProperty(ctx, shillconst.ServicePropertyCellularServingOperator)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get last good APN info")
	}
	uuid, ok := servingOperatorMap[shillconst.OperatorUUIDKey]
	if !ok {
		return "", "", errors.New("serving operator UUID not found")
	}
	code, ok = servingOperatorMap[shillconst.OperatorCode]
	if !ok {
		return "", "", errors.New("service operator code not found")
	}
	return uuid, code, nil
}

// getCellularDeviceProperty gets a shill device string property
func (h *Helper) getCellularDeviceProperty(ctx context.Context, propertyName string) (string, error) {
	props, err := h.Device.GetProperties(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get Device properties")
	}
	info, err := props.GetString(propertyName)
	if err != nil {
		return "", errors.Wrapf(err, "error getting property %q", propertyName)
	}

	return info, nil
}

// GetIMEIFromShill gets the current modem IMEI from shill.
func (h *Helper) GetIMEIFromShill(ctx context.Context) (string, error) {
	return h.getCellularDeviceProperty(ctx, shillconst.DevicePropertyCellularIMEI)
}

// GetIMSIFromShill gets the current modem IMSI from shill.
func (h *Helper) GetIMSIFromShill(ctx context.Context) (string, error) {
	return h.getCellularDeviceProperty(ctx, shillconst.DevicePropertyCellularIMSI)
}

// SetServiceProvidersExclusiveOverride adds an override MODB to shill.
// The function returns a closure to delete the override file.
func SetServiceProvidersExclusiveOverride(ctx context.Context, sourceFile string) (func(), error) {
	input, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %q", sourceFile)
	}
	err = ioutil.WriteFile(shillconst.ServiceProviderOverridePath, input, 0644)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to write %q", shillconst.ServiceProviderOverridePath)
	}
	return func() {
		os.Remove(shillconst.ServiceProviderOverridePath)
	}, nil
}

// getCellularServiceDictProperty gets a shill service dictionary property
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

	dictProp, ok := info.(map[string]string)
	if !ok {
		return nil, errors.Wrapf(err, "invalid format for %q", propertyName)
	}

	return dictProp, nil
}

// getCellularServiceProperty gets a shill service dictionary property
func (h *Helper) getCellularServiceProperty(ctx context.Context, propertyName string) (string, error) {
	// Verify that a connectable Cellular service exists and ensure it is connected.
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return "", errors.Wrap(err, "unable to find Cellular Service for Device")
	}
	props, err := service.GetShillProperties(ctx)
	if err != nil {
		return "", errors.Wrap(err, "error getting Service properties")
	}
	info, err := props.GetString(propertyName)
	if err != nil {
		return "", errors.Wrapf(err, "error getting property %q", propertyName)
	}

	return info, nil
}

// SetAPN sets the Custom/OTHER APN property `Cellular.APN`.
func (h *Helper) SetAPN(ctx context.Context, apn map[string]string) error {
	ctx, st := timing.Start(ctx, "Helper.SetApn")
	defer st.End()

	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get Cellular Service")
	}
	if err := service.SetProperty(ctx, shillconst.ServicePropertyCellularAPN, apn); err != nil {
		return errors.Wrap(err, "failed to set Cellular.APN")
	}
	return nil
}

// GetCellularLastAttachAPN gets Cellular.LastAttachAPN dictionary from shill properties.
func (h *Helper) GetCellularLastAttachAPN(ctx context.Context) (map[string]string, error) {
	return h.getCellularServiceDictProperty(ctx, shillconst.ServicePropertyCellularLastAttachAPN)
}

// GetCellularLastGoodAPN gets Cellular.LastAttachAPN dictionary from shill properties.
func (h *Helper) GetCellularLastGoodAPN(ctx context.Context) (map[string]string, error) {
	return h.getCellularServiceDictProperty(ctx, shillconst.ServicePropertyCellularLastGoodAPN)
}

// GetCurrentIPType returns current ip_type of LastGoodAPN.
func (h *Helper) GetCurrentIPType(ctx context.Context) (string, error) {
	apnInfo, err := h.getCellularServiceDictProperty(ctx, shillconst.ServicePropertyCellularLastGoodAPN)
	if err != nil {
		return "", errors.Wrap(err, "failed to get last good APN info")
	}

	ipType := apnInfo["ip_type"]
	if len(ipType) > 0 {
		return ipType, nil
	}
	return "ipv4", nil
}

// GetNetworkProvisionedCellularIPTypes returns the currently provisioned IP types
func (h *Helper) GetNetworkProvisionedCellularIPTypes(ctx context.Context) (ipv4, ipv6 bool, err error) {
	// Verify that a connectable Cellular service exists and ensure it is connected.
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return false, false, errors.Wrap(err, "unable to find Cellular Service")
	}
	if isConnected, err := service.IsConnected(ctx); err != nil {
		return false, false, errors.Wrap(err, "unable to get IsConnected for Service")
	} else if !isConnected {
		if _, err := h.ConnectToDefault(ctx); err != nil {
			return false, false, errors.Wrap(err, "unable to Connect to default service")
		}
	}
	configs, err := service.GetIPConfigs(ctx)
	if err != nil {
		return false, false, errors.Wrap(err, "failed to get IPConfigs from service")
	}

	ipv4Present := false
	ipv6Present := false
	for _, config := range configs {
		props, err := config.GetIPProperties(ctx)
		if err != nil {
			return false, false, errors.Wrap(err, "failed to get IPConfig properties")
		}
		testing.ContextLog(ctx, "Address :", props.Address)
		ip := net.ParseIP(props.Address)
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			testing.ContextLog(ctx, "IPv4 Address :", props.Address, " len(ip): ", len(ip))
			ipv4Present = true
		} else {
			testing.ContextLog(ctx, "IPv6 Address :", props.Address, " len(ip): ", len(ip))
			ipv6Present = true
		}
	}
	if ipv4Present == false && ipv6Present == false {
		return false, false, errors.New("no IP networks provisioned")
	}
	return ipv4Present, ipv6Present, nil
}

// GetCurrentICCID gets current ICCID
func (h *Helper) GetCurrentICCID(ctx context.Context) (string, error) {
	return h.getCellularServiceProperty(ctx, shillconst.ServicePropertyCellularICCID)
}

// GetCurrentNetworkName gets current Network name
func (h *Helper) GetCurrentNetworkName(ctx context.Context) (string, error) {
	return h.getCellularServiceProperty(ctx, shillconst.ServicePropertyName)
}

// disableNonCellularInterfaceforTesting disable all non cellular interfaces
func (h *Helper) disableNonCellularInterfaceforTesting(ctx context.Context) error {
	ctx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime*2)
	defer cancel()
	// Disable Ethernet and/or WiFi if present and defer re-enabling.
	if enableFunc, err := h.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet); err != nil {
		return errors.Wrap(err, "unable to disable Ethernet")
	} else if enableFunc != nil {
		h.enableEthernetFunc = enableFunc
	}
	if enableFunc, err := h.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyWifi); err != nil {
		return errors.Wrap(err, "unable to disable Wifi")
	} else if enableFunc != nil {
		h.enableWifiFunc = enableFunc
	}
	return nil
}

// enablePreviouslyDisabledNonCellularInterfaceforTesting enable previously disabled interfaces
func (h *Helper) enablePreviouslyDisabledNonCellularInterfaceforTesting(ctx context.Context) {
	if h.enableEthernetFunc != nil {
		h.enableEthernetFunc(ctx)
	}
	if h.enableWifiFunc != nil {
		h.enableWifiFunc(ctx)
	}
	h.enableEthernetFunc = nil
	h.enableWifiFunc = nil
}

// RunTestOnCellularInterface setup the device for cellular tests.
func (h *Helper) RunTestOnCellularInterface(ctx context.Context, testBody func(ctx context.Context) error) error {
	// Verify that a connectable Cellular service exists and ensure it is connected.
	service, err := h.FindServiceForDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to find Cellular Service")
	}
	if isConnected, err := service.IsConnected(ctx); err != nil {
		return errors.Wrap(err, "unable to get IsConnected for Service")
	} else if !isConnected {
		if _, err := h.ConnectToDefault(ctx); err != nil {
			return errors.Wrap(err, "unable to Connect to default service")
		}
	}

	if err := h.disableNonCellularInterfaceforTesting(ctx); err != nil {
		return errors.Wrap(err, "failed to disable non cellular interface")
	}
	defer h.enablePreviouslyDisabledNonCellularInterfaceforTesting(ctx)

	// wait for portal checks to pass and to be online.
	if err := service.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 30*time.Second); err != nil {
		return errors.Wrapf(err, "%s is not online", service)
	}

	return testBody(ctx)
}

// PrintHostInfoLabels prints the host info labels
func (h *Helper) PrintHostInfoLabels(ctx context.Context) {
	for _, label := range h.Labels {
		testing.ContextLog(ctx, "Labels :", label)
	}
}

// PrintModemInfo prints modem details
func (h *Helper) PrintModemInfo(ctx context.Context) {
	testing.ContextLog(ctx, "Modem Type : ", h.modemInfo.Type)
	testing.ContextLog(ctx, "Modem IMEI : ", h.modemInfo.IMEI)
	testing.ContextLog(ctx, "Modem Supported Bands : ", h.modemInfo.SupportedBands)
	testing.ContextLog(ctx, "Modem SIM Count : ", h.modemInfo.SimCount)
}

// PrintSIMInfo prints SIM details
func (h *Helper) PrintSIMInfo(ctx context.Context) {
	for _, s := range h.simInfo {
		testing.ContextLog(ctx, "SIM Slot ID   : ", s.SlotID)
		testing.ContextLog(ctx, "SIM Type      : ", s.Type)
		testing.ContextLog(ctx, "SIM Test eSIM : ", s.TestEsim)
		testing.ContextLog(ctx, "SIM EID       : ", s.EID)
		for _, p := range s.ProfileInfo {
			testing.ContextLog(ctx, "SIM Profile ICCID : ", p.ICCID)
			testing.ContextLog(ctx, "SIM Profile PIN : ", p.SimPin)
			testing.ContextLog(ctx, "SIM Profile PUK : ", p.SimPuk)
			testing.ContextLog(ctx, "SIM Profile Carrier Name : ", p.CarrierName)
		}
	}
}

// GetHostInfoLabels reads the labels from autotest_host_info_labels
func (h *Helper) GetHostInfoLabels(ctx context.Context, labels []string) error {

	dims := make(LabelMap)
	for _, label := range labels {
		val := strings.SplitN(label, ":", 2)
		switch len(val) {
		case 1:
			dims[val[0]] = append(dims[val[0]], "")
		case 2:
			dims[val[0]] = append(dims[val[0]], val[1])
		}
	}
	h.Labels = labels
	h.PrintHostInfoLabels(ctx)
	h.modemInfo = GetModemInfoFromHostInfoLabels(ctx, dims)
	h.simInfo = GetSIMInfoFromHostInfoLabels(ctx, dims)
	h.carrierName = GetCellularCarrierFromHostInfoLabels(ctx, dims)
	testing.ContextLog(ctx, "Carrier Name : ", h.carrierName)
	h.devicePools = GetDevicePoolFromHostInfoLabels(ctx, dims)
	testing.ContextLog(ctx, "Pools : ", h.devicePools)
	return nil
}

// GetPINAndPUKForICCID returns the pin and puk info for the given iccid from host_info_label
func (h *Helper) GetPINAndPUKForICCID(ctx context.Context, iccid string) (string, string, error) {
	for _, s := range h.simInfo {
		for _, p := range s.ProfileInfo {
			if p.ICCID == iccid {
				return p.SimPin, p.SimPuk, nil
			}
		}
	}
	return "", "", nil
}

// GetLabelCarrierName return the current carrier name
func (h *Helper) GetLabelCarrierName(ctx context.Context) string {
	return h.carrierName
}
