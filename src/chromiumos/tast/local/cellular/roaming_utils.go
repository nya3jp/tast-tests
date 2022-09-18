// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/network"
	"chromiumos/tast/testing"
)

var uiPollOptions = testing.PollOptions{
	Timeout:  60 * time.Second,
	Interval: 500 * time.Millisecond,
}

// ConfigureRoamingNetwork enables roaming on DUT and connects to a cellular network if there isn't an active network
func ConfigureRoamingNetwork(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (string, error) {
	var networkName string
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		return networkName, errors.Wrap(err, "could not find MM dbus object with a valid sim")
	}

	helper, err := NewHelper(ctx)
	if err != nil {
		return networkName, errors.Wrap(err, "failed to create cellular.Helper")
	}

	cleanup1, err := helper.InitDeviceProperty(ctx, shillconst.DevicePropertyCellularPolicyAllowRoaming, true)
	if err != nil {
		return networkName, errors.Wrap(err, "could not set PolicyAllowRoaming")
	}
	defer cleanup1(ctx)

	cleanup2, err := helper.InitServiceProperty(ctx, shillconst.ServicePropertyCellularAllowRoaming, true)
	if err != nil {
		return networkName, errors.Wrap(err, "could not set AllowRoaming property")
	}
	defer cleanup2(ctx)

	networkName, err = helper.GetCurrentNetworkName(ctx)
	if err != nil {
		return networkName, errors.Wrap(err, "could not get name")
	}

	if len(networkName) == 0 {
		cellularNetworkProvider, err := network.NewCellularNetworkProvider(ctx, false)
		if err != nil {
			return networkName, errors.Wrap(err, "failed to create cellular network provider")
		}

		pSIMNetworkNames, err := cellularNetworkProvider.PSimNetworkNames(ctx)
		if err != nil {
			return networkName, errors.Wrap(err, "failed to fetch pSIM network names")
		}

		eSIMNetworkNames, err := cellularNetworkProvider.ESimNetworkNames(ctx)
		if err != nil {
			return networkName, errors.Wrap(err, "failed to fetch eSIM network names")
		}

		if len(pSIMNetworkNames) > 0 {
			networkName = pSIMNetworkNames[0]
		} else {
			networkName = eSIMNetworkNames[0]
		}

		ui := uiauto.New(tconn)

		app, err := ossettings.OpenNetworkDetailPage(ctx, tconn, cr, networkName)
		if err != nil {
			return networkName, errors.Wrap(err, "failed to open network detail page")
		}
		defer app.Close(ctx)

		if err := uiauto.Combine("",
			ui.WaitUntilExists(ossettings.ConnectButton),
			ui.LeftClick(ossettings.ConnectButton),
			ui.WaitUntilExists(ossettings.ConnectedStatus),
		)(ctx); err != nil {
			return networkName, errors.Wrap(err, "failed to connect to network")
		}
	}
	return networkName, nil
}
