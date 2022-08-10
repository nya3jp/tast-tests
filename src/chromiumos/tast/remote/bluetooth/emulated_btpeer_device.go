// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"fmt"

	cbt "chromiumos/tast/common/chameleon/devices/common/bluetooth"
	"chromiumos/tast/errors"
	bts "chromiumos/tast/services/cros/bluetooth"
	"chromiumos/tast/testing"
)

type emulatedBTPeerDeviceCache struct {
	advertisedName            string
	localBluetoothAddress     string
	pinCode                   string
	capabilities              map[string]interface{}
	hasPinCode                bool
	supportsInitConnect       bool
	supportedTransportMethods []cbt.TransportMethod
	classOfService            int
	classOfDevice             int
	authenticationMode        cbt.AuthenticationMode
	deviceType                cbt.DeviceType
	port                      string
}

// EmulatedBTPeerDevice is a wrapper around the BluezPeripheral Chameleond
// bluetooth device interface which can be used to preform common actions
// related to bluetooth devices at a higher level than the interface.
//
// It also caches key data about the emulated device for ease of use. This data
// is initially cached when an EmulatedBTPeerDevice is created with
// NewEmulatedBTPeerDevice. The cached data can be refreshed by calling
// RefreshCache manually.
type EmulatedBTPeerDevice struct {
	rpc   cbt.BluezPeripheral
	cache *emulatedBTPeerDeviceCache
}

// NewEmulatedBTPeerDevice created a new EmulatedBTPeerDevice given the provided
// RPC interface to the device. The device will be initialized and key data
// about the device will be cached for later reference.
//
// Note: A btpeer can only act as one device at a time. If you require multiple
// devices at once, use different btpeers.
func NewEmulatedBTPeerDevice(ctx context.Context, deviceRPC cbt.BluezPeripheral) (*EmulatedBTPeerDevice, error) {
	d := &EmulatedBTPeerDevice{
		rpc: deviceRPC,
	}
	if err := d.initializeEmulatedBTPeerDevice(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *EmulatedBTPeerDevice) initializeEmulatedBTPeerDevice(ctx context.Context) error {
	testing.ContextLog(ctx, "Preparing device for use")

	if err := d.rpc.Init(ctx, false); err != nil {
		return errors.Wrap(err, "failed to initialize device")
	}

	connected, err := d.rpc.CheckSerialConnection(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check serial connection")
	}
	if !connected {
		return errors.New("failed serial connection check")
	}

	success, err := d.rpc.EnterCommandMode(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to enter command mode")
	}
	if !success {
		return errors.New("failed to enter command mode")
	}

	testing.ContextLog(ctx, "Collecting device info")
	if err := d.RefreshCache(ctx); err != nil {
		return errors.Wrap(err, "failed to collect device info")
	}

	if err := d.rpc.SpecifyDeviceType(ctx, d.cache.deviceType.String()); err != nil {
		return errors.Wrapf(err, "failed to specify the btpeer to act as a %q device", d.cache.deviceType.String())
	}

	testing.ContextLogf(ctx, "Successfully initialized %s", d.String())
	return nil
}

// RefreshCache refreshes all local cached data retrieved from the device and
// any cached calculations that depend on such data.
func (d *EmulatedBTPeerDevice) RefreshCache(ctx context.Context) error {
	var err error
	d.cache = &emulatedBTPeerDeviceCache{}
	if err = d.evaluateCapabilities(ctx); err != nil {
		return errors.Wrap(err, "failed to evaluate device capabilities")
	}
	if d.cache.advertisedName, err = d.rpc.GetAdvertisedName(ctx); err != nil {
		return errors.Wrap(err, "failed to get device name")
	}
	if d.cache.localBluetoothAddress, err = d.rpc.GetLocalBluetoothAddress(ctx); err != nil {
		return errors.Wrap(err, "failed to get device address")
	}
	if deviceType, err := d.rpc.GetDeviceType(ctx); err == nil {
		d.cache.deviceType = cbt.DeviceType(deviceType)
	} else {
		return errors.Wrap(err, "failed to get device type")
	}
	if d.cache.port, err = d.rpc.GetPort(ctx); err != nil {
		return errors.Wrap(err, "failed to get device port")
	}
	if d.cache.hasPinCode {
		if d.cache.pinCode, err = d.rpc.GetPinCode(ctx); err != nil {
			return errors.Wrap(err, "failed to get device pin")
		}
	}
	// Collect information not supported by LE devices.
	if len(d.SupportedTransportMethods()) == 1 && d.SupportsTransportMethodLE() {
		if d.cache.classOfService, err = d.rpc.GetClassOfService(ctx); err != nil {
			return errors.Wrap(err, "failed to get device class of service")
		}
		if d.cache.classOfDevice, err = d.rpc.GetClassOfDevice(ctx); err != nil {
			return errors.Wrap(err, "failed to get device class of device")
		}
		if authenticationMode, err := d.rpc.GetAuthenticationMode(ctx); err == nil {
			d.cache.authenticationMode = cbt.AuthenticationMode(authenticationMode)
		} else {
			return errors.Wrap(err, "failed to get device authentication mode")
		}
	}
	return nil
}

// String returns a string representation of this EmulatedBTPeerDevice with
// key identifiable information included.
func (d *EmulatedBTPeerDevice) String() string {
	return fmt.Sprintf("EmulatedBTPeerDevice(Type=%q,Addr=%q,Name=%q)", d.cache.deviceType, d.cache.localBluetoothAddress, d.cache.advertisedName)
}

// evaluateCapabilities calls RPC().GetCapabilities and caches the result. It
// also checks for known DeviceCapability options and stores their values for
// later reference.
func (d *EmulatedBTPeerDevice) evaluateCapabilities(ctx context.Context) error {
	// Refresh stored capabilities.
	capabilities, err := d.rpc.GetCapabilities(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get device capabilities")
	}
	d.cache.capabilities = capabilities

	// Evaluate hasPinCode.
	hasPinCode, ok := d.cache.capabilities[cbt.DeviceCapabilityHasPin.String()]
	if !ok {
		return errors.Errorf("device capabilities missing %q", cbt.DeviceCapabilityHasPin)
	}
	hasPinCodeBool, ok := hasPinCode.(bool)
	if !ok {
		return errors.Errorf("expected device capability %q to be a bool, got %v", cbt.DeviceCapabilityHasPin, hasPinCode)
	}
	d.cache.hasPinCode = hasPinCodeBool

	// Evaluate supportsInitConnect.
	supportsInitConnect, ok := d.cache.capabilities[cbt.DeviceCapabilityInitConnect.String()]
	if !ok {
		return errors.Errorf("device capabilities missing %q", cbt.DeviceCapabilityInitConnect)
	}
	supportsInitConnectBool, ok := supportsInitConnect.(bool)
	if !ok {
		return errors.Errorf("expected device capability %q to be a bool, got %v", cbt.DeviceCapabilityInitConnect, supportsInitConnect)
	}
	d.cache.supportsInitConnect = supportsInitConnectBool

	// Evaluate supportedTransportMethods.
	supportedTransportMethods, ok := d.cache.capabilities[cbt.DeviceCapabilityTransports.String()]
	if !ok {
		return errors.Errorf("device capabilities missing %q", cbt.DeviceCapabilityTransports)
	}
	supportedTransportMethodsSlice, ok := supportedTransportMethods.([]interface{})
	if !ok {
		return errors.Errorf("expected device capability %q to be an []interface{}, got %v", cbt.DeviceCapabilityTransports, supportedTransportMethods)
	}
	for _, method := range supportedTransportMethodsSlice {
		methodStr, ok := method.(string)
		if !ok {
			return errors.Errorf("expected device capability %q to be an []interface{} of strings, got %v", cbt.DeviceCapabilityTransports, supportedTransportMethods)
		}
		d.cache.supportedTransportMethods = append(d.cache.supportedTransportMethods, cbt.TransportMethod(methodStr))
	}

	return nil
}

// RPC returns the Chameleond RPC interface for this device as a
// BluezPeripheral.
func (d *EmulatedBTPeerDevice) RPC() cbt.BluezPeripheral {
	return d.rpc
}

// RPCMouse returns the Chameleond RPC interface for this device as a
// MousePeripheral.
//
// This should only be used if the RPC interface provided to
// NewEmulatedBTPeerDevice when this EmulatedBTPeerDevice was created was a
// MousePeripheral. Otherwise, calling this method will cause a panic.
func (d *EmulatedBTPeerDevice) RPCMouse() cbt.MousePeripheral {
	return d.rpc.(cbt.MousePeripheral)
}

// RPCPhone returns the Chameleond RPC interface for this device as a
// PhonePeripheral.
//
// This should only be used if the RPC interface provided to
// NewEmulatedBTPeerDevice when this EmulatedBTPeerDevice was created was a
// PhonePeripheral. Otherwise, calling this method will cause a panic.
func (d *EmulatedBTPeerDevice) RPCPhone() cbt.PhonePeripheral {
	return d.rpc.(cbt.PhonePeripheral)
}

// RPCKeyboard returns the Chameleond RPC interface for this device as a
// KeyboardPeripheral.
//
// This should only be used if the RPC interface provided to
// NewEmulatedBTPeerDevice when this EmulatedBTPeerDevice was created was a
// KeyboardPeripheral. Otherwise, calling this method will cause a panic.
func (d *EmulatedBTPeerDevice) RPCKeyboard() cbt.KeyboardPeripheral {
	return d.rpc.(cbt.KeyboardPeripheral)
}

// RPCFastPair returns the Chameleond RPC interface for this device as a
// FastPairPeripheral.
//
// This should only be used if the RPC interface provided to
// NewEmulatedBTPeerDevice when this EmulatedBTPeerDevice was created was a
// FastPairPeripheral. Otherwise, calling this method will cause a panic.
func (d *EmulatedBTPeerDevice) RPCFastPair() cbt.FastPairPeripheral {
	return d.rpc.(cbt.FastPairPeripheral)
}

// AdvertisedName returns the cached result of
// RPC().GetAdvertisedName().
func (d *EmulatedBTPeerDevice) AdvertisedName() string {
	return d.cache.advertisedName
}

// LocalBluetoothAddress returns the cached result of
// RPC().GetLocalBluetoothAddress().
func (d *EmulatedBTPeerDevice) LocalBluetoothAddress() string {
	return d.cache.localBluetoothAddress
}

// PinCode returns the cached result of
// RPC().GetPinCode().
func (d *EmulatedBTPeerDevice) PinCode() string {
	return d.cache.pinCode
}

// Capabilities returns the cached result of
// RPC().GetCapabilities().
func (d *EmulatedBTPeerDevice) Capabilities() map[string]interface{} {
	return d.cache.capabilities
}

// HasPinCode returns true if the device has a pin code configured.
//
// Specifically, this is a cached result of evaluating the cached Capabilities
// of this device.
func (d *EmulatedBTPeerDevice) HasPinCode() bool {
	return d.cache.hasPinCode
}

// SupportsInitConnect returns true if the device is capable of initiating a
// bluetooth connection.
//
// Specifically, this is a cached result of evaluating the cached Capabilities
// of this device.
func (d *EmulatedBTPeerDevice) SupportsInitConnect() bool {
	return d.cache.supportsInitConnect
}

// SupportedTransportMethods returns the transport methods this device is
// capable of.
//
// Specifically, this is a cached result of evaluating the cached Capabilities
// of this device.
func (d *EmulatedBTPeerDevice) SupportedTransportMethods() []cbt.TransportMethod {
	return d.cache.supportedTransportMethods
}

// SupportsTransportMethod returns true if the device supports the given
// transport method.
//
// Specifically, this is a cached result of evaluating the cached Capabilities
// of this device.
func (d *EmulatedBTPeerDevice) SupportsTransportMethod(method cbt.TransportMethod) bool {
	for _, supportedMethod := range d.cache.supportedTransportMethods {
		if supportedMethod.String() == method.String() {
			return true
		}
	}
	return false
}

// SupportsTransportMethodLE returns true if the device supports the LE
// transport method. This is a shortcut to calling SupportsTransportMethod for
// the method.
func (d *EmulatedBTPeerDevice) SupportsTransportMethodLE() bool {
	return d.SupportsTransportMethod(cbt.TransportMethodLE)
}

// SupportsTransportMethodBREDR returns true if the device supports the BREDR
// transport method. This is a shortcut to calling SupportsTransportMethod for
// the method.
func (d *EmulatedBTPeerDevice) SupportsTransportMethodBREDR() bool {
	return d.SupportsTransportMethod(cbt.TransportMethodBREDR)
}

// ClassOfService returns the cached result of
// RPC().GetClassOfService().
//
// Note: This will not be set for LE-only devices.
func (d *EmulatedBTPeerDevice) ClassOfService() int {
	return d.cache.classOfService
}

// ClassOfDevice returns the cached result of
// RPC().GetClassOfDevice().
//
// Note: This will not be set for LE-only devices.
func (d *EmulatedBTPeerDevice) ClassOfDevice() int {
	return d.cache.classOfDevice
}

// AuthenticationMode returns the cached result of
// RPC().GetAuthenticationMode().
//
// Note: This will not be set for LE-only devices.
func (d *EmulatedBTPeerDevice) AuthenticationMode() cbt.AuthenticationMode {
	return d.cache.authenticationMode
}

// DeviceType returns the cached result of
// RPC().GetDeviceType().
func (d *EmulatedBTPeerDevice) DeviceType() cbt.DeviceType {
	return d.cache.deviceType
}

// BTSDevice creates a BTTestService Device from this device's cached
// data.
func (d *EmulatedBTPeerDevice) BTSDevice() *bts.Device {
	return &bts.Device{
		MacAddress:     d.cache.localBluetoothAddress,
		AdvertisedName: d.cache.advertisedName,
		HasPinCode:     d.cache.hasPinCode,
		PinCode:        d.cache.pinCode,
	}
}
