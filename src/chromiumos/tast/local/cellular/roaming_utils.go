// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/network"
	"chromiumos/tast/testing"
)

var uiPollOptions = testing.PollOptions{
	Timeout:  60 * time.Second,
	Interval: 500 * time.Millisecond,
}

// SetRoamingPolicy configures the roaming policy
func SetRoamingPolicy(ctx context.Context, allowRoaming, autoConnect bool) error {
	_, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		return errors.Wrap(err, "could not find MM dbus object with a valid sim")
	}

	helper, err := NewHelper(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create cellular.Helper")
	}

	_, err = helper.InitServiceProperty(ctx, shillconst.ServicePropertyAutoConnect, autoConnect)
	if err != nil {
		return errors.Wrap(err, "could not initialize autoconnect to false")
	}

	_, err = helper.InitDeviceProperty(ctx, shillconst.DevicePropertyCellularPolicyAllowRoaming, allowRoaming)
	if err != nil {
		return errors.Wrap(err, "could not set PolicyAllowRoaming to true")
	}

	_, err = helper.InitServiceProperty(ctx, shillconst.ServicePropertyCellularAllowRoaming, allowRoaming)
	if err != nil {
		return errors.Wrap(err, "could not set AllowRoaming property to true")
	}

	return nil
}

// ConnectToCellularNetwork connects to a cellular network if there isn't an active network
func ConnectToCellularNetwork(ctx context.Context) error {
	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		return errors.Wrap(err, "could not find MM dbus object with a valid sim")
	}

	helper, err := NewHelper(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create cellular.Helper")
	}

	service, err := helper.FindServiceForDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "could not find default service for device")
	}

	isConnected, err := service.IsConnected(ctx)
	if err != nil {
		return errors.Wrap(err, "could not check if service is connected")
	}
	if isConnected {
		if err := service.Disconnect(ctx); err != nil {
			return errors.Wrap(err, "failed to disconnect from roaming network prior to starting the actual test")
		}
	}

	if err := modem.WaitForState(ctx, mmconst.ModemStateRegistered, time.Minute); err != nil {
		return errors.Wrap(err, "Modem is not registered")
	}

	if err := helper.ConnectToService(ctx, service); err != nil {
		return errors.Wrap(err, "Unable to connect to roaming service")
	}

	return nil
}

// GetCellularNetwork returns the nick name of the current active network
func GetCellularNetwork(ctx context.Context) (string, error) {
	var networkName string
	helper, err := NewHelper(ctx)
	if err != nil {
		return networkName, errors.Wrap(err, "failed to create cellular.Helper")
	}

	iccid, err := helper.GetCurrentICCID(ctx)
	if err != nil {
		return networkName, errors.Wrap(err, "failed to fetch current network iccid")
	}

	cellularNetworkProvider, err := network.NewCellularNetworkProvider(ctx, false)
	if err != nil {
		return networkName, errors.Wrap(err, "failed to create cellular network provider")
	}

	networkName, err = cellularNetworkProvider.GetNetworkNameByIccid(ctx, iccid)
	if networkName == "" {
		return networkName, errors.Wrap(err, "failed to fetch network name by iccid")
	}

	return networkName, nil
}
