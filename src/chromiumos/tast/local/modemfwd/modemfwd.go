// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package modemfwd interacts with modemfwd D-Bus service.
package modemfwd

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	dbusPath      = "/org/chromium/Modemfwd"
	dbusName      = "org.chromium.Modemfwd"
	dbusInterface = "org.chromium.Modemfwd"
	// JobName is the name of the modemfwd process
	JobName = "modemfwd"
	// DisableAutoUpdatePref disables auto update on modemfwd
	DisableAutoUpdatePref = "/var/lib/modemfwd/disable_auto_update"
	// PurgeDlcsDelay is the time modemfwd waits until it starts cleaning up the DLCs
	PurgeDlcsDelay = 2 * time.Minute
)

// Modemfwd is used to interact with the modemfwd process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/modemfwd/dbus_bindings/org.chromium.Modemfwd.xml
type Modemfwd struct {
	*dbusutil.DBusObject
}

// New connects to modemfwd via D-Bus and returns a Modemfwd object.
func New(ctx context.Context) (*Modemfwd, error) {
	obj, err := dbusutil.NewDBusObject(ctx, dbusName, dbusInterface, dbusPath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect to modemfwd")
	}
	return &Modemfwd{obj}, nil
}

// ForceFlash calls modemfwd's ForceFlash D-Bus method and waits for the modem to reappear.
func (m *Modemfwd) ForceFlash(ctx context.Context, device string, options map[string]interface{}) error {
	forceFlash := func(ctx context.Context) error {
		result := false
		if err := m.Call(ctx, "ForceFlash", device, options).Store(&result); err != nil {
			return err
		}
		if !result {
			return errors.New("ForceFlash returned false")
		}
		return nil
	}
	return executeFunctionAndWaitForQuiescence(ctx, forceFlash)
}

// UpdateFirmwareCompletedSignal holds values created from the MemoryPressureChrome D-Bus
// signal.
type UpdateFirmwareCompletedSignal struct {
	success bool
	errStr  string
}

func executeFunctionAndWaitForQuiescence(ctx context.Context, function func(ctx context.Context) error) error {
	watcher, err := WatchUpdateFirmwareCompleted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to watch for UpdateFirmwareCompleted")
	}
	defer watcher.Close(ctx)

	if err = function(ctx); err != nil {
		return errors.Wrap(err, "failed to execute function")
	}

	// Map D-Bus signals into UpdateFirmwareCompletedSignal.
	select {
	case sig := <-watcher.Signals:
		signal, err := parseUpdateFirmwareCompletedSignal(sig)
		if err != nil {
			return errors.Wrap(err, "signal returned error")
		}
		if signal.errStr != "" {
			return errors.New("modemfwd returned failure: " + signal.errStr)
		}
		return nil
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "didn't get UpdateFirmwareCompleted D-Bus signal")
	}
}

func parseUpdateFirmwareCompletedSignal(sig *dbus.Signal) (UpdateFirmwareCompletedSignal, error) {
	res := UpdateFirmwareCompletedSignal{}
	if len(sig.Body) != 2 {
		return res, errors.Errorf("expected 2 params, got %d", len(sig.Body))
	}
	success, ok := sig.Body[0].(bool)
	if !ok {
		return res, errors.Errorf("unable to convert 'success' from bool %v", sig.Body[0])
	}
	errStr, ok := sig.Body[1].(string)
	if !ok {
		return res, errors.Errorf("unable to convert 'errStr' from string %v", sig.Body[1])
	}
	res.success = success
	res.errStr = errStr
	return res, nil
}

// Stop stops the Modem Firmware Daemon if it is currently running and returns true if it was stopped.
func Stop(ctx context.Context) (bool, error) {
	if !upstart.JobExists(ctx, JobName) {
		return false, nil
	}
	_, _, pid, err := upstart.JobStatus(ctx, JobName)
	if err != nil {
		return false, errors.Wrapf(err, "failed to run upstart.JobStatus for %q", JobName)
	}
	if pid == 0 {
		return false, nil
	}
	err = upstart.StopJob(ctx, JobName)
	if err != nil {
		return false, errors.Wrapf(err, "failed to stop %q", JobName)
	}
	return true, nil
}

// StartAndWaitForQuiescence starts the modemfwd job and waits for the initial sequence to complete
// or until an error is logged.
func StartAndWaitForQuiescence(ctx context.Context) error {
	startJob := func(ctx context.Context) error {
		err := upstart.StartJob(ctx, JobName, upstart.WithArg("DEBUG_MODE", "true"))
		if err != nil {
			return errors.Wrapf(err, "failed to start %q", JobName)
		}
		return nil
	}
	return executeFunctionAndWaitForQuiescence(ctx, startJob)
}

// WatchUpdateFirmwareCompleted returns a SignalWatcher to observe the
// "UpdateFirmwareCompleted" signal.
func WatchUpdateFirmwareCompleted(ctx context.Context) (*dbusutil.SignalWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    "UpdateFirmwareCompleted",
	}
	return dbusutil.NewSignalWatcherForSystemBus(ctx, spec)
}

// DisableAutoUpdate sets the modemfwd pref value to 1, to disable auto updates. The function
// returns a closure to restore the pref to its original state.
func DisableAutoUpdate(ctx context.Context) (func(), error) {
	fileExists := disableAutoUpdatePrefFileExists()
	currentValue := GetAutoUpdatePrefValue(ctx)
	if err := ioutil.WriteFile(DisableAutoUpdatePref, []byte("1"), 0666); err != nil {
		return nil, errors.Wrapf(err, "could not write to %s", DisableAutoUpdatePref)
	}
	return func() {
		if !fileExists {
			os.Remove(DisableAutoUpdatePref)
		} else if !currentValue {
			ioutil.WriteFile(DisableAutoUpdatePref, []byte("0"), 0666)
		}
	}, nil
}

func disableAutoUpdatePrefFileExists() bool {
	_, err := os.Stat(DisableAutoUpdatePref)
	return !os.IsNotExist(err)
}

// GetAutoUpdatePrefValue Gets the pref value of DisableAutoUpdatePref.
// True if the file exists and it's set to 1, false otherwise.
func GetAutoUpdatePrefValue(ctx context.Context) bool {
	if !disableAutoUpdatePrefFileExists() {
		return false
	}
	pref, err := ioutil.ReadFile(DisableAutoUpdatePref)
	if err != nil {
		return false
	}
	if bytes.Compare(pref, []byte("1")) == 0 {
		return true
	}
	return false
}

// Device prefixes
const (
	usbPrefix = "usb:"
	pciPrefix = "pci:"
	socPrefix = "soc:"
)

// WaitForDevice waits for a device to be present with a given ID |DEVICE-TYPE-PREFIX:VID:PID|.
// Note: For SoC devices, there is no VID/PID; the modem is assumed to be a QRTR device and is
// detected by looking for a QRTR node implemented a set of modem QMI services.
func WaitForDevice(ctx context.Context, deviceID string) error {
	if strings.HasPrefix(deviceID, usbPrefix) {
		return WaitForUsbDevice(ctx, deviceID, time.Minute)
	} else if strings.HasPrefix(deviceID, pciPrefix) {
		return WaitForPciDevice(ctx, deviceID, time.Minute)
	} else if strings.HasPrefix(deviceID, socPrefix) {
		return WaitForQrtrModemDevice(ctx, time.Minute)
	}
	return errors.Errorf("failed to find device, unknown device prefix in ID: %q", deviceID)
}

// WaitForPciDevice polls for the presence of the PCI device with ID |VID:PID|
// Note: This expects both the VID an PID to be provided and will not work for wildcard or partial
// IDs such as "*:*", or "ffff:".
func WaitForPciDevice(ctx context.Context, pciID string, maxWaitTime time.Duration) error {
	// trim device-type prefix from id if it was passed in with one
	pciID = strings.TrimPrefix(pciID, pciPrefix)
	// trim location and any other tags after the device name
	pciID = strings.Split(pciID, " ")[0]

	// Check whether PCI device presented as |VID:PID| exists in host
	// lspci will still return a zero exit code even if the device is not found
	// so we need to check that the output contains the provided pciID.
	if err := testing.Poll(ctx, func(context.Context) error {
		if output, err := testexec.CommandContext(ctx, "lspci", "-n", "-d", pciID).Output(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "unexpected PCI status in host os")
		} else if !strings.Contains(string(output), pciID) {
			return errors.New("PCI device not found")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  maxWaitTime,
		Interval: 500 * time.Millisecond,
	}); err != nil {
		return errors.Wrapf(err, "failed to find PCI device with ID: %q", pciID)
	}
	return nil
}

// WaitForUsbDevice polls for the presence of the USB device with ID |VID:PID|
func WaitForUsbDevice(ctx context.Context, usbID string, maxWaitTime time.Duration) error {
	// trim device-type prefix from id if it was passed in with one
	usbID = strings.TrimPrefix(usbID, usbPrefix)
	// trim location and any other tags after the device name
	usbID = strings.Split(usbID, " ")[0]

	if err := testing.Poll(ctx, func(context.Context) error {
		// Check whether USB device presented as |VID:PID| exists in host
		// If the specified device is not found, a non-zero exit code is returned
		// by lsusb and err will not be nil
		// |err == nil| indicates |lsusb -d XXXX:XXXX| finds the expected devices.
		if err := testexec.CommandContext(ctx, "lsusb", "-d", usbID).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "unexpected usb status in host os")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  maxWaitTime,
		Interval: 500 * time.Millisecond,
	}); err != nil {
		return errors.Wrapf(err, "failed to find USB device with ID: %q", usbID)
	}
	return nil
}

type qmiService int

const (
	qmiWirelessDataService     qmiService = 1
	qmiDeviceManagementService            = 2
	qmiNetworkAccessService               = 3
)

// requiredQmiServices are the QMI services required for a QRTR node to represent a modem
// See: src/third_party/modemmanager-next/src/mm-qrtr-bus-watcher.c
var requiredQmiServices = []qmiService{
	qmiWirelessDataService,
	qmiDeviceManagementService,
	qmiNetworkAccessService,
}

type qrtrNode struct {
	services map[qmiService]bool
}

// WaitForQrtrModemDevice polls for the presence of a QRTR node implementing the required modem QMI services.
func WaitForQrtrModemDevice(ctx context.Context, maxWaitTime time.Duration) error {
	if err := testing.Poll(ctx, func(context.Context) error {
		if ok, err := hasQrtrNodeWithModemServices(ctx, requiredQmiServices...); err != nil {
			return testing.PollBreak(err)
		} else if !ok {
			return errors.New("failed to find QRTR node with the requested services")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  maxWaitTime,
		Interval: 500 * time.Millisecond,
	}); err != nil {
		return errors.Wrap(err, "failed to find a QRTR modem device")
	}
	return nil
}

// hasQrtrNodeWithModemServices searches for a QRTR node implementing all of the requested QMI services and returns an error if none is found.
func hasQrtrNodeWithModemServices(ctx context.Context, requiredServices ...qmiService) (bool, error) {
	nodes, err := getQrtrNodes(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get available QRTR nodes")
	}

	for _, node := range nodes {
		foundAll := true
		for _, serviceID := range requiredServices {
			if !node.services[serviceID] {
				foundAll = false
				break
			}
		}
		if foundAll {
			return true, nil
		}
	}

	return false, nil
}

// getQrtrNodes returns the available QRTR nodes returned by qrtr-lookup
func getQrtrNodes(ctx context.Context) (map[int]*qrtrNode, error) {
	out, err := testexec.CommandContext(ctx, "qrtr-lookup").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get QRTR services via qrtr-lookup")
	}

	// collect all nodes from qrtr-lookup
	const fieldCount = 6
	const serviceIndex = 0
	const nodeIndex = 3
	nodes := make(map[int]*qrtrNode, 0)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < fieldCount {
			continue
		}

		nodeID, err := strconv.Atoi(fields[nodeIndex])
		if err != nil {
			continue
		}

		serviceID, err := strconv.Atoi(fields[serviceIndex])
		if err != nil {
			continue
		}

		if _, ok := nodes[nodeID]; !ok {
			nodes[nodeID] = &qrtrNode{services: make(map[qmiService]bool)}
		}
		nodes[nodeID].services[qmiService(serviceID)] = true
	}

	return nodes, nil
}
