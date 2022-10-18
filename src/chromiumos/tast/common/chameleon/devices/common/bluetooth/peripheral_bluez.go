// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/common/xmlrpc"
)

// BluezPeripheral is an interface for making RPC calls to a chameleond daemon
// targeting a specific bluetooth peripheral chameleon device flow.
//
// This is based off of the Python class "chameleond.utils.bluetooth_raspi.BluezPeripheral"
// from the chameleon source. Refer to that source for more complete
// documentation.
type BluezPeripheral interface {
	PeripheralKit

	// StartPairingAgent calls the Chameleond RPC method of the same name.
	// Starts a pairing agent with the specified capability.
	StartPairingAgent(ctx context.Context, capability string) error

	// StopPairingAgent calls the Chameleond RPC method of the same name.
	// Stops the pairing agent.
	StopPairingAgent(ctx context.Context) error

	// EnableBLE calls the Chameleond RPC method of the same name.
	// Puts this device into either LE or Classic bluetooth mode.
	EnableBLE(ctx context.Context, useBLE bool) error

	// GetBaseDeviceType calls the Chameleond RPC method of the same name.
	// Returns the base device type of a peripheral, i.e. "BLE_MOUSE" -> "MOUSE".
	GetBaseDeviceType(ctx context.Context, deviceType string) (string, error)

	// SpecifyDeviceType calls the Chameleond RPC method of the same name.
	// Instantiates one of the supported devices specified by deviceType.
	SpecifyDeviceType(ctx context.Context, deviceType string) error

	// SetBtdFlags calls the Chameleond RPC method of the same name.
	// Allows bluetoothd config execution flags to be set.
	SetBtdFlags(ctx context.Context, deviceType string) error

	// ResetStack calls the Chameleond RPC method of the same name.
	// Restores the BT stack to a pristine state by restarting running services.
	ResetStack(ctx context.Context, nextDeviceType string) error

	// Init calls the Chameleond RPC method of the same name.
	// Ensures the chip is in the correct state for the tests to be run.
	// Returns true if successful.
	Init(ctx context.Context, factoryReset bool) error

	// CleanCachedFiles calls the Chameleond RPC method of the same name.
	// Cleans up files that bluetoothd loads when starts.
	CleanCachedFiles(ctx context.Context) (bool, error)

	// AdapterPowerOff calls the Chameleond RPC method of the same name.
	// Powers off the bluez adapter.
	// Returns true if successful.
	AdapterPowerOff(ctx context.Context) (bool, error)

	// AdapterPowerOn calls the Chameleond RPC method of the same name.
	// Powers on the bluez adapter.
	// Returns true if successful.
	AdapterPowerOn(ctx context.Context) (bool, error)

	// SetAdapterAlias calls the Chameleond RPC method of the same name.
	// Sets the bluez adapter alias to name.
	SetAdapterAlias(ctx context.Context, name string) error

	// AdvertiseWithNamesAndAddresses calls the Chameleond RPC method of the same
	// name.
	// Advertises local names and addresses for duration time one by one.
	// After this function returned, the local name and address will be reset back
	// to default and discoverable will be turned off.
	//
	// The namesAndAddresses parameter is a list of tuples (name, addr). Each
	// tuple describes the local name and the address for the device to advertise.
	AdvertiseWithNamesAndAddresses(ctx context.Context, namesAndAddresses [][]string, advertiseDurationSec int) error

	// GetDeviceWithAddress calls the Chameleond RPC method of the same name.
	// Gets the bluez device name that matches the given MAC address.
	GetDeviceWithAddress(ctx context.Context, addr string) (string, error)

	// RemoveDevice calls the Chameleond RPC method of the same name.
	// Removes a remote device from bluez that matches the given MAC address.
	RemoveDevice(ctx context.Context, remoteAddress string) error

	// StartDiscovery calls the Chameleond RPC method of the same name.
	// Tries to start discovery on the bluez adapter.
	// Returns true if successful.
	StartDiscovery(ctx context.Context) (bool, error)

	// StopDiscovery calls the Chameleond RPC method of the same name.
	// Tries to stop discovery on the adapter.
	// Returns true if successful.
	StopDiscovery(ctx context.Context) (bool, error)

	// StartUnfilteredDiscovery calls the Chameleond RPC method of the same name.
	// Starts unfiltered discovery session for DUT advertisement testing.
	// Returns true if successful.
	StartUnfilteredDiscovery(ctx context.Context) (bool, error)

	// StopUnfilteredDiscovery calls the Chameleond RPC method of the same name.
	// Stops unfiltered discovery session for DUT advertisement testing
	// Returns true if successful.
	StopUnfilteredDiscovery(ctx context.Context) (bool, error)

	// FindAdvertisementWithAttributes calls the Chameleond RPC method of the same name.
	// Locates an advertisement containing the requested attributes from btmon.
	FindAdvertisementWithAttributes(ctx context.Context, attrs []string, timeoutSec int) (advertisingEvent string, err error)

	// SendHIDReport calls the Chameleond RPC method of the same name.
	// Sends a hid report to the bluez service.
	SendHIDReport(ctx context.Context, report int) error
}

// CommonBluezPeripheral is a base implementation of BluezPeripheral that
// provides methods for making XMLRPC calls to a chameleond daemon.
// See the BluezPeripheral interface for more detailed documentation.
type CommonBluezPeripheral struct {
	CommonPeripheralKit
}

// NewCommonBluezPeripheral creates a new instance of CommonBluezPeripheral.
func NewCommonBluezPeripheral(xmlrpcClient *xmlrpc.XMLRpc, methodNamePrefix string) *CommonBluezPeripheral {
	return &CommonBluezPeripheral{
		CommonPeripheralKit: *NewCommonPeripheralKit(xmlrpcClient, methodNamePrefix),
	}
}

// StartPairingAgent calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.StartPairingAgent, see that for more details.
func (c *CommonBluezPeripheral) StartPairingAgent(ctx context.Context, capability string) error {
	return c.RPC("StartPairingAgent").Args(capability).Call(ctx)
}

// StopPairingAgent calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.StopPairingAgent, see that for more details.
func (c *CommonBluezPeripheral) StopPairingAgent(ctx context.Context) error {
	return c.RPC("StopPairingAgent").Call(ctx)
}

// EnableBLE calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.EnableBLE, see that for more details.
func (c *CommonBluezPeripheral) EnableBLE(ctx context.Context, useBLE bool) error {
	return c.RPC("EnableBLE").Args(useBLE).Call(ctx)
}

// GetBaseDeviceType calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.GetBaseDeviceType, see that for more details.
func (c *CommonBluezPeripheral) GetBaseDeviceType(ctx context.Context, deviceType string) (string, error) {
	return c.RPC("GetBaseDeviceType").Args(deviceType).CallForString(ctx)
}

// SpecifyDeviceType calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.SpecifyDeviceType, see that for more details.
func (c *CommonBluezPeripheral) SpecifyDeviceType(ctx context.Context, deviceType string) error {
	return c.RPC("SpecifyDeviceType").Args(deviceType).Call(ctx)
}

// SetBtdFlags calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.SetBtdFlags, see that for more details.
func (c *CommonBluezPeripheral) SetBtdFlags(ctx context.Context, deviceType string) error {
	return c.RPC("SetBtdFlags").Args(deviceType).Call(ctx)
}

// ResetStack calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.ResetStack, see that for more details.
func (c *CommonBluezPeripheral) ResetStack(ctx context.Context, nextDeviceType string) error {
	return c.RPC("ResetStack").Args(nextDeviceType).Call(ctx)
}

// Init calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.Init, see that for more details.
func (c *CommonBluezPeripheral) Init(ctx context.Context, factoryReset bool) error {
	return c.RPC("Init").Args(factoryReset).Call(ctx)
}

// CleanCachedFiles calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.CleanCachedFiles, see that for more details.
func (c *CommonBluezPeripheral) CleanCachedFiles(ctx context.Context) (bool, error) {
	return c.RPC("CleanCachedFiles").CallForBool(ctx)
}

// AdapterPowerOff calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.AdapterPowerOff, see that for more details.
func (c *CommonBluezPeripheral) AdapterPowerOff(ctx context.Context) (bool, error) {
	return c.RPC("AdapterPowerOff").CallForBool(ctx)
}

// AdapterPowerOn calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.AdapterPowerOn, see that for more details.
func (c *CommonBluezPeripheral) AdapterPowerOn(ctx context.Context) (bool, error) {
	return c.RPC("AdapterPowerOn").CallForBool(ctx)
}

// SetAdapterAlias calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.SetAdapterAlias, see that for more details.
func (c *CommonBluezPeripheral) SetAdapterAlias(ctx context.Context, name string) error {
	return c.RPC("SetAdapterAlias").Args(name).Call(ctx)
}

// AdvertiseWithNamesAndAddresses calls the Chameleond RPC method of the same
// name. This implements BluezPeripheral.AdvertiseWithNamesAndAddresses, see
// that for more details.
func (c *CommonBluezPeripheral) AdvertiseWithNamesAndAddresses(ctx context.Context, namesAndAddresses [][]string, advertiseDurationSec int) error {
	return c.RPC("AdvertiseWithNamesAndAddresses").Args(namesAndAddresses, advertiseDurationSec).Call(ctx)
}

// GetDeviceWithAddress calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.GetDeviceWithAddress, see that for more
// details.
func (c *CommonBluezPeripheral) GetDeviceWithAddress(ctx context.Context, addr string) (string, error) {
	return c.RPC("GetDeviceWithAddress").Args(addr).CallForString(ctx)
}

// RemoveDevice calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.RemoveDevice, see that for more details.
func (c *CommonBluezPeripheral) RemoveDevice(ctx context.Context, remoteAddress string) error {
	return c.RPC("RemoveDevice").Args(remoteAddress).Call(ctx)
}

// StartDiscovery calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.StartDiscovery, see that for more details.
func (c *CommonBluezPeripheral) StartDiscovery(ctx context.Context) (bool, error) {
	return c.RPC("StartDiscovery").CallForBool(ctx)
}

// StopDiscovery calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.StopDiscovery, see that for more details.
func (c *CommonBluezPeripheral) StopDiscovery(ctx context.Context) (bool, error) {
	return c.RPC("StopDiscovery").CallForBool(ctx)
}

// StartUnfilteredDiscovery calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.StartUnfilteredDiscovery, see that for more
// details.
func (c *CommonBluezPeripheral) StartUnfilteredDiscovery(ctx context.Context) (bool, error) {
	return c.RPC("StartUnfilteredDiscovery").CallForBool(ctx)
}

// StopUnfilteredDiscovery calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.StopUnfilteredDiscovery, see that for more
// details.
func (c *CommonBluezPeripheral) StopUnfilteredDiscovery(ctx context.Context) (bool, error) {
	return c.RPC("StopUnfilteredDiscovery").CallForBool(ctx)
}

// FindAdvertisementWithAttributes calls the Chameleond RPC method of the same
// name. This implements BluezPeripheral.FindAdvertisementWithAttributes, see
// that for more details.
func (c *CommonBluezPeripheral) FindAdvertisementWithAttributes(ctx context.Context, attrs []string, timeoutSec int) (advertisingEvent string, err error) {
	return c.RPC("FindAdvertisementWithAttributes").Args(attrs, timeoutSec).CallForString(ctx)
}

// SendHIDReport calls the Chameleond RPC method of the same name.
// This implements BluezPeripheral.SendHIDReport, see that for more details.
func (c *CommonBluezPeripheral) SendHIDReport(ctx context.Context, report int) error {
	return c.RPC("SendHIDReport").Args(report).Call(ctx)
}
