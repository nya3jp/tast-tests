// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/common/chameleon/devices"
	"chromiumos/tast/common/xmlrpc"
)

// PeripheralKit is an interface for making RPC calls to a chameleond daemon
// targeting a specific peripheral chameleon device flow.
//
// This is based off of the Python class "chameleond.utils.peripheral_kit.PeripheralKit"
// from the chameleon source. Refer to that source for more complete
// documentation.
type PeripheralKit interface {
	devices.ChameleonDeviceFlow

	// GetPort calls the Chameleond RPC method of the same name.
	// Get the tty device path of the serial port.
	GetPort(ctx context.Context) (string, error)

	// GetCapabilities calls the Chameleond RPC method of the same name.
	// Returns the capabilities of the kit that tests should account for.
	GetCapabilities(ctx context.Context) (map[string]interface{}, error)

	// EnterCommandMode calls the Chameleond RPC method of the same name.
	// Makes the kit enter command mode.
	// Returns true if successful.
	EnterCommandMode(ctx context.Context) (bool, error)

	// LeaveCommandMode calls the Chameleond RPC method of the same name.
	// Makes the kit leave command mode.
	// Returns true if successful.
	LeaveCommandMode(ctx context.Context) (bool, error)

	// Reboot calls the Chameleond RPC method of the same name.
	// Reboots (or partially reset) the kit.
	Reboot(ctx context.Context) error

	// FactoryReset calls the Chameleond RPC method of the same name.
	// Resets the kit to the factory defaults.
	// Returns true if successful.
	FactoryReset(ctx context.Context) (bool, error)

	// PowerCycle calls the Chameleond RPC method of the same name.
	// Power cycles the USB port where kit is attached.
	// Returns true if successful.
	PowerCycle(ctx context.Context) (bool, error)

	// GetAdvertisedName calls the Chameleond RPC method of the same name.
	// Returns the name advertised by the kit to other bluetooth devices.
	GetAdvertisedName(ctx context.Context) (string, error)

	// GetFirmwareVersion calls the Chameleond RPC method of the same name.
	// Returns the firmware version of the kit.
	GetFirmwareVersion(ctx context.Context) (string, error)

	// GetOperationMode calls the Chameleond RPC method of the same name.
	// Returns the operation mode of the kit.
	GetOperationMode(ctx context.Context) (string, error)

	// SetCentralMode calls the Chameleond RPC method of the same name.
	// Sets the kit to central mode.
	// Returns true if successful.
	SetCentralMode(ctx context.Context) (bool, error)

	// SetPeripheralMode calls the Chameleond RPC method of the same name.
	// Sets the kit to peripheral mode.
	// Returns true if successful.
	SetPeripheralMode(ctx context.Context) (bool, error)

	// GetAuthenticationMode calls the Chameleond RPC method of the same name.
	// Returns the authentication mode of the kit.
	GetAuthenticationMode(ctx context.Context) (string, error)

	// SetAuthenticationMode calls the Chameleond RPC method of the same name.
	// Sets the authentication mode to the specified mode.
	// Returns true if successful.
	SetAuthenticationMode(ctx context.Context, mode string) (bool, error)

	// GetPinCode calls the Chameleond RPC method of the same name.
	// Returns the pin code for kit authentication.
	GetPinCode(ctx context.Context) (string, error)

	// SetPinCode calls the Chameleond RPC method of the same name.
	// Sets the pin code for kit authentication.
	// Returns true if successful.
	SetPinCode(ctx context.Context, pinCode string) (bool, error)

	// GetServiceProfile calls the Chameleond RPC method of the same name.
	// Returns the service profile currently in use by the kit.
	GetServiceProfile(ctx context.Context) error

	// SetServiceProfileSPP calls the Chameleond RPC method of the same name.
	// Sets SPP as the service profile.
	// Returns true if successful.
	SetServiceProfileSPP(ctx context.Context) (bool, error)

	// SetServiceProfileHID calls the Chameleond RPC method of the same name.
	// Sets HID as the service profile.
	// Returns true if successful.
	SetServiceProfileHID(ctx context.Context) (bool, error)

	// GetLocalBluetoothAddress calls the Chameleond RPC method of the same name.
	// Returns the local (kit's) bluetooth MAC address.
	GetLocalBluetoothAddress(ctx context.Context) (string, error)

	// GetConnectionStatus calls the Chameleond RPC method of the same name.
	// Returns the connection status of the kit.
	// If the status is true, it indicates that the kit is connected to a remote
	// device, usually the DUT.
	GetConnectionStatus(ctx context.Context) (bool, error)

	// EnableConnectionStatusMessage calls the Chameleond RPC method of the same
	// name.
	// Enables the connection status message.
	// Returns true if successful.
	EnableConnectionStatusMessage(ctx context.Context) (bool, error)

	// DisableConnectionStatusMessage calls the Chameleond RPC method of the same
	// name.
	// Disables the connection status message.
	// Returns true if successful.
	DisableConnectionStatusMessage(ctx context.Context) (bool, error)

	// GetRemoteConnectedBluetoothAddress calls the Chameleond RPC method of the
	// same name.
	// Returns the bluetooth MAC address of the current connected remote host.
	GetRemoteConnectedBluetoothAddress(ctx context.Context) (string, error)

	// GetDeviceType calls the Chameleond RPC method of the same name.
	// Returns a string representing the HID type of the kit.
	GetDeviceType(ctx context.Context) (string, error)

	// SetHIDType calls the Chameleond RPC method of the same name.
	// Sets HID type to the specified device type.
	// Returns true if successful.
	SetHIDType(ctx context.Context, deviceType string) (bool, error)

	// GetClassOfService calls the Chameleond RPC method of the same name.
	// Returns the class of the service, if supported, which is usually a number
	// assigned by the bluetooth SIG.
	GetClassOfService(ctx context.Context) (int, error)

	// SetClassOfService calls the Chameleond RPC method of the same name.
	// Sets the class of service, if supported, which is usually a number assigned
	// by the bluetooth SIG.
	// Returns true if the class of service was set successfully, or if this
	// action is not supported.
	SetClassOfService(ctx context.Context, classOfService int) (bool, error)

	// SetDefaultClassOfService calls the Chameleond RPC method of the same name.
	// Sets the default class of service, if supported.
	// Returns true if the class of service was set to the default successfully,
	// or if this action is not supported.
	SetDefaultClassOfService(ctx context.Context) (bool, error)

	// GetClassOfDevice calls the Chameleond RPC method of the same name.
	// Returns the class of device, if supported, which is usually a number
	// assigned by the bluetooth SIG.
	GetClassOfDevice(ctx context.Context) (int, error)

	// SetClassOfDevice calls the Chameleond RPC method of the same name.
	// Sets the class of device, if supported, which is usually a number assigned
	// by the bluetooth SIG.
	// Returns true if the class of device was set successfully, or if this
	// action is not supported.
	SetClassOfDevice(ctx context.Context, deviceType int) (bool, error)

	// SetRemoteAddress calls the Chameleond RPC method of the same name.
	// Sets the remote bluetooth MAC address.
	// Returns true if successful.
	SetRemoteAddress(ctx context.Context, remoteAddress string) (bool, error)

	// Connect calls the Chameleond RPC method of the same name.
	// Connects to the stored remote bluetooth address.
	// Returns true if connecting to the stored remote address succeeded, or
	// false if a timeout occurs.
	Connect(ctx context.Context) (bool, error)

	// ConnectToRemoteAddress calls the Chameleond RPC method of the same name.
	// Connects to the remote bluetooth MAC address.
	// Returns true if connecting to the remote address succeeded.
	ConnectToRemoteAddress(ctx context.Context, remoteAddress string) (bool, error)

	// Disconnect calls the Chameleond RPC method of the same name.
	// Disconnects from the remote device. Specifically, this causes the
	// peripheral emulation kit to disconnect from the remote connected device,
	// usually the DUT.
	// Returns true if disconnecting from the remote device succeeded.
	Disconnect(ctx context.Context) (bool, error)

	// Discover calls the Chameleond RPC method of the same name.
	// Discovers the remote bluetooth MAC address.
	// Returns true if discovering the remote address succeeded.
	Discover(ctx context.Context, remoteAddress string) (bool, error)

	// SetDiscoverable calls the Chameleond RPC method of the same name.
	// Sets the discoverability of the device.
	SetDiscoverable(ctx context.Context, discoverable bool) error

	// Close calls the Chameleond RPC method of the same name.
	// Attempts to close the device gracefully.
	// Returns true if successful.
	Close(ctx context.Context) (bool, error)
}

// CommonPeripheralKit is a base implementation of PeripheralKit that
// provides methods for making XMLRPC calls to a chameleond daemon.
// See the PeripheralKit interface for more detailed documentation.
type CommonPeripheralKit struct {
	devices.CommonChameleonDeviceFlow
}

// NewCommonPeripheralKit creates a new instance of CommonPeripheralKit.
func NewCommonPeripheralKit(xmlrpcClient *xmlrpc.XMLRpc, methodNamePrefix string) *CommonPeripheralKit {
	return &CommonPeripheralKit{
		CommonChameleonDeviceFlow: *devices.NewCommonChameleonDeviceFlow(xmlrpcClient, methodNamePrefix),
	}
}

// GetPort calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetPort, see that for more details.
func (c *CommonPeripheralKit) GetPort(ctx context.Context) (string, error) {
	return c.RPC("GetPort").CallForString(ctx)
}

// GetCapabilities calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetCapabilities, see that for more details.
func (c *CommonPeripheralKit) GetCapabilities(ctx context.Context) (map[string]interface{}, error) {
	capabilities := make(map[string]interface{})
	err := c.RPC("GetCapabilities").Returns(&capabilities).Call(ctx)
	if err != nil {
		return nil, err
	}
	return capabilities, nil
}

// EnterCommandMode calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.EnterCommandMode, see that for more details.
func (c *CommonPeripheralKit) EnterCommandMode(ctx context.Context) (bool, error) {
	return c.RPC("EnterCommandMode").CallForBool(ctx)
}

// LeaveCommandMode calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.LeaveCommandMode, see that for more details.
func (c *CommonPeripheralKit) LeaveCommandMode(ctx context.Context) (bool, error) {
	return c.RPC("LeaveCommandMode").CallForBool(ctx)
}

// Reboot calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.Reboot, see that for more details.
func (c *CommonPeripheralKit) Reboot(ctx context.Context) error {
	return c.RPC("Reboot").Call(ctx)
}

// FactoryReset calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.FactoryReset, see that for more details.
func (c *CommonPeripheralKit) FactoryReset(ctx context.Context) (bool, error) {
	return c.RPC("FactoryReset").CallForBool(ctx)
}

// PowerCycle calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.PowerCycle, see that for more details.
func (c *CommonPeripheralKit) PowerCycle(ctx context.Context) (bool, error) {
	return c.RPC("PowerCycle").CallForBool(ctx)
}

// GetAdvertisedName calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetAdvertisedName, see that for more details.
func (c *CommonPeripheralKit) GetAdvertisedName(ctx context.Context) (string, error) {
	return c.RPC("GetAdvertisedName").CallForString(ctx)
}

// GetFirmwareVersion calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetFirmwareVersion, see that for more details.
func (c *CommonPeripheralKit) GetFirmwareVersion(ctx context.Context) (string, error) {
	return c.RPC("GetFirmwareVersion").CallForString(ctx)
}

// GetOperationMode calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetOperationMode, see that for more details.
func (c *CommonPeripheralKit) GetOperationMode(ctx context.Context) (string, error) {
	return c.RPC("GetOperationMode").CallForString(ctx)
}

// SetCentralMode calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetCentralMode, see that for more details.
func (c *CommonPeripheralKit) SetCentralMode(ctx context.Context) (bool, error) {
	return c.RPC("SetCentralMode").CallForBool(ctx)
}

// SetPeripheralMode calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetPeripheralMode, see that for more details.
func (c *CommonPeripheralKit) SetPeripheralMode(ctx context.Context) (bool, error) {
	return c.RPC("SetPeripheralMode").CallForBool(ctx)
}

// GetAuthenticationMode calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetAuthenticationMode, see that for more details.
func (c *CommonPeripheralKit) GetAuthenticationMode(ctx context.Context) (string, error) {
	return c.RPC("GetAuthenticationMode").CallForString(ctx)
}

// SetAuthenticationMode calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetAuthenticationMode, see that for more
// details.
func (c *CommonPeripheralKit) SetAuthenticationMode(ctx context.Context, mode string) (bool, error) {
	return c.RPC("SetAuthenticationMode").Args(mode).CallForBool(ctx)
}

// GetPinCode calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetPinCode, see that for more details.
func (c *CommonPeripheralKit) GetPinCode(ctx context.Context) (string, error) {
	return c.RPC("GetPinCode").CallForString(ctx)
}

// SetPinCode calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetPinCode, see that for more details.
func (c *CommonPeripheralKit) SetPinCode(ctx context.Context, pinCode string) (bool, error) {
	return c.RPC("SetPinCode").Args(pinCode).CallForBool(ctx)
}

// GetServiceProfile calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetServiceProfile, see that for more details.
func (c *CommonPeripheralKit) GetServiceProfile(ctx context.Context) error {
	return c.RPC("GetServiceProfile").Call(ctx)
}

// SetServiceProfileSPP calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetServiceProfileSPP, see that for more
// details.
func (c *CommonPeripheralKit) SetServiceProfileSPP(ctx context.Context) (bool, error) {
	return c.RPC("SetServiceProfileSPP").CallForBool(ctx)
}

// SetServiceProfileHID calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetServiceProfileHID, see that for more
// details.
func (c *CommonPeripheralKit) SetServiceProfileHID(ctx context.Context) (bool, error) {
	return c.RPC("SetServiceProfileHID").CallForBool(ctx)
}

// GetLocalBluetoothAddress calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetLocalBluetoothAddress, see that for more
// details.
func (c *CommonPeripheralKit) GetLocalBluetoothAddress(ctx context.Context) (string, error) {
	return c.RPC("GetLocalBluetoothAddress").CallForString(ctx)
}

// GetConnectionStatus calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetConnectionStatus, see that for more details.
func (c *CommonPeripheralKit) GetConnectionStatus(ctx context.Context) (bool, error) {
	return c.RPC("GetConnectionStatus").CallForBool(ctx)
}

// EnableConnectionStatusMessage calls the Chameleond RPC method of the same
// name. This implements PeripheralKit.EnableConnectionStatusMessage, see that
// for more details.
func (c *CommonPeripheralKit) EnableConnectionStatusMessage(ctx context.Context) (bool, error) {
	return c.RPC("EnableConnectionStatusMessage").CallForBool(ctx)
}

// DisableConnectionStatusMessage calls the Chameleond RPC method of the same
// name. This implements PeripheralKit.DisableConnectionStatusMessage, see that
// for more details.
func (c *CommonPeripheralKit) DisableConnectionStatusMessage(ctx context.Context) (bool, error) {
	return c.RPC("DisableConnectionStatusMessage").CallForBool(ctx)
}

// GetRemoteConnectedBluetoothAddress calls the Chameleond RPC method of the
// same name. This implements PeripheralKit.GetRemoteConnectedBluetoothAddress,
// see that for more details.
func (c *CommonPeripheralKit) GetRemoteConnectedBluetoothAddress(ctx context.Context) (string, error) {
	return c.RPC("GetRemoteConnectedBluetoothAddress").CallForString(ctx)
}

// GetDeviceType calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetDeviceType, see that for more details.
func (c *CommonPeripheralKit) GetDeviceType(ctx context.Context) (string, error) {
	return c.RPC("GetDeviceType").CallForString(ctx)
}

// SetHIDType calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetHIDType, see that for more details.
func (c *CommonPeripheralKit) SetHIDType(ctx context.Context, deviceType string) (bool, error) {
	return c.RPC("SetHIDType").Args(deviceType).CallForBool(ctx)
}

// GetClassOfService calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetClassOfService, see that for more details.
func (c *CommonPeripheralKit) GetClassOfService(ctx context.Context) (int, error) {
	return c.RPC("GetClassOfService").CallForInt(ctx)
}

// SetClassOfService calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetClassOfService, see that for more details.
func (c *CommonPeripheralKit) SetClassOfService(ctx context.Context, classOfService int) (bool, error) {
	return c.RPC("SetClassOfService").Args(classOfService).CallForBool(ctx)
}

// SetDefaultClassOfService calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetDefaultClassOfService, see that for more
// details.
func (c *CommonPeripheralKit) SetDefaultClassOfService(ctx context.Context) (bool, error) {
	return c.RPC("SetDefaultClassOfService").CallForBool(ctx)
}

// GetClassOfDevice calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.GetClassOfDevice, see that for more details.
func (c *CommonPeripheralKit) GetClassOfDevice(ctx context.Context) (int, error) {
	return c.RPC("GetClassOfDevice").CallForInt(ctx)
}

// SetClassOfDevice calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetClassOfDevice, see that for more details.
func (c *CommonPeripheralKit) SetClassOfDevice(ctx context.Context, deviceType int) (bool, error) {
	return c.RPC("SetClassOfDevice").Args(deviceType).CallForBool(ctx)
}

// SetRemoteAddress calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetRemoteAddress, see that for more details.
func (c *CommonPeripheralKit) SetRemoteAddress(ctx context.Context, remoteAddress string) (bool, error) {
	return c.RPC("SetRemoteAddress").Args(remoteAddress).CallForBool(ctx)
}

// Connect calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.Connect, see that for more details.
func (c *CommonPeripheralKit) Connect(ctx context.Context) (bool, error) {
	return c.RPC("Connect").CallForBool(ctx)
}

// ConnectToRemoteAddress calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.ConnectToRemoteAddress, see that for more
// details.
func (c *CommonPeripheralKit) ConnectToRemoteAddress(ctx context.Context, remoteAddress string) (bool, error) {
	return c.RPC("ConnectToRemoteAddress").Args(remoteAddress).CallForBool(ctx)
}

// Disconnect calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.Disconnect, see that for more details.
func (c *CommonPeripheralKit) Disconnect(ctx context.Context) (bool, error) {
	return c.RPC("Disconnect").CallForBool(ctx)
}

// Discover calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.Discover, see that for more details.
func (c *CommonPeripheralKit) Discover(ctx context.Context, remoteAddress string) (bool, error) {
	return c.RPC("Discover").Args(remoteAddress).CallForBool(ctx)
}

// SetDiscoverable calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.SetDiscoverable, see that for more details.
func (c *CommonPeripheralKit) SetDiscoverable(ctx context.Context, discoverable bool) error {
	return c.RPC("SetDiscoverable").Args(discoverable).Call(ctx)
}

// Close calls the Chameleond RPC method of the same name.
// This implements PeripheralKit.Close, see that for more details.
func (c *CommonPeripheralKit) Close(ctx context.Context) (bool, error) {
	return c.RPC("Close").CallForBool(ctx)
}
