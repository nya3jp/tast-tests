// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cellular provides functions for testing Cellular connectivity.
package cellular

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
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

// Suspend - Put system into suspended state.
// Returns duration, error
func (h *Helper) Suspend(ctx context.Context, wakeUpTimeout time.Duration, checkEarlyWake bool) (time.Duration, error) {
	const (
		powerdDBusSuspendPath = "/usr/bin/powerd_dbus_suspend"
		rtcPath               = "/sys/class/rtc/rtc0/since_epoch"
		pauseEthernetHookPath = "/run/autotest_pause_ethernet_hook"
	)

	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to lock the check network hook")
	}
	defer unlock()

	rtcTimeSeconds := func() (int, error) {
		b, err := ioutil.ReadFile(rtcPath)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to read the %s", rtcPath)
		}
		return strconv.Atoi(strings.TrimSpace(string(b)))
	}

	if wakeUpTimeout < 2*time.Second {
		// May cause DUT not wake from sleep if the suspend time is 1 second.
		// It happens when the current clock (floating point) is close to the
		// next integer, as the RTC sysfs interface only accepts integers.
		// Make sure it is larger than or equal to 2.
		return 0, errors.Errorf("unexpected wake up timeout: got %s, want >= 2 seconds", wakeUpTimeout)
	}

	startRTC, err := rtcTimeSeconds()
	if err != nil {
		return 0, err
	}

	wakeUpTimeoutSecond := int(wakeUpTimeout.Seconds())
	if err := testexec.CommandContext(ctx, powerdDBusSuspendPath,
		"--delay=0", // By default it delays the start of suspending by a second.
		fmt.Sprintf("--wakeup_timeout=%d", wakeUpTimeoutSecond),  // Ask the powerd_dbus_suspend to spawn a RTC alarm to wake the DUT up after wakeUpTimeoutSecond.
		fmt.Sprintf("--suspend_for_sec=%d", wakeUpTimeoutSecond), // Request powerd daemon to suspend for wakeUpTimeoutSecond.
	).Run(); err != nil {
		return 0, err
	}

	finishRTC, err := rtcTimeSeconds()
	if err != nil {
		return 0, err
	}

	suspendedInterval := finishRTC - startRTC
	testing.ContextLogf(ctx, "RTC suspend time: %d", suspendedInterval)

	if checkEarlyWake && suspendedInterval < wakeUpTimeoutSecond {
		return 0, errors.Errorf("the DUT wakes up too early, got %d, want %d", suspendedInterval, wakeUpTimeoutSecond)
	}

	return time.Duration(suspendedInterval) * time.Second, nil
}
