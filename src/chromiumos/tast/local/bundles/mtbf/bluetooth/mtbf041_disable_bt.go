// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/bluetooth/btconn"
	"chromiumos/tast/local/bundles/mtbf/bluetooth/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// const btDeviceName = "Magic Mouse"
// const btDeviceName = "Office speaker"
// const btDeviceName = "SonicGear A. P-V"
// const btDeviceName = "Keyboard K370/K375"

const (
	a2dpVarName   = "bt.a2dp.deviceName"
	hidVarName    = "bt.hid.deviceName"
	notApplicable = "N/A"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF041DisableBT,
		Desc:         "MTBF041 Disable/enable Bluetooth actually disables/enables",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Vars:         []string{"bt.a2dp.deviceName", "bt.hid.deviceName"},
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"xliu@cienet.com"},
	})
}

// MTBF041DisableBT case verifies the Enable/Disable Bluetooth functionality for A2DP and HID
func MTBF041DisableBT(ctx context.Context, s *testing.State) {
	s.Logf("bt varName: %v, %v", a2dpVarName, hidVarName)
	a2dpDevName := utils.GetVar(ctx, s, a2dpVarName)
	hidDevName := utils.GetVar(ctx, s, hidVarName)

	if a2dpDevName == notApplicable {
		s.Fatal(mtbferrors.New(mtbferrors.BTA2DPNeeded, nil, "MTBF041"))
	}

	if hidDevName == notApplicable {
		s.Fatal(mtbferrors.New(mtbferrors.BTHIDNeeded, nil, "MTBF041"))
	}

	btConn := initBluetooth(ctx, s)
	defer btConn.Close()
	testing.Sleep(ctx, 2*time.Second)
	a2dpDevAddr, hidDevAddr := getBtDeviceAddr(ctx, s, btConn, a2dpDevName, hidDevName)
	s.Logf("a2dp, hid BT device names: %v, %v", a2dpDevName, hidDevName)
	s.Logf("a2dp, hid BT device address: %v, %v", a2dpDevAddr, hidDevAddr)

	if err := btConn.SwitchOff(); err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	s.Log("BT disabled")
	testing.Sleep(ctx, 5*time.Second)
	enableBluetooth(ctx, s, btConn, a2dpDevAddr, hidDevAddr)

	s.Log("BT enabled")
	testing.Sleep(ctx, 5*time.Second)
	btConn.EnterBtPage()

	// if err := btConsole.Connect(a2dpDevAddr); err != nil {
	// 	s.Fatal("MTBF failed: ", err)
	// }

	if connected, err := btConn.CheckBtDevice(a2dpDevName); err != nil {
		s.Fatal("MTBF failed: ", err)
	} else if !connected {
		s.Fatal(mtbferrors.New(mtbferrors.BTConnected, nil, a2dpDevName))
	}

	// if err := btConsole.Connect(hidDevAddr); err != nil {
	// 	s.Fatal("MTBF failed: ", err)
	// }

	if connected, err := btConn.CheckBtDevice(hidDevName); err != nil {
		s.Fatal("MTBF failed: ", err)
	} else if !connected {
		s.Fatal(mtbferrors.New(mtbferrors.BTConnected, nil, hidDevName))
	}
}

// initBluetooth initializes the Bluetooth connection
func initBluetooth(ctx context.Context, s *testing.State) *btconn.BtConn {
	var cr = s.PreValue().(*chrome.Chrome)
	btConn, err := btconn.New(ctx, s, cr, nil)

	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	return btConn
}

// getBtDeviceAddr retrieves Bt connection addresses for A2DP and HID
func getBtDeviceAddr(ctx context.Context, s *testing.State, btConn *btconn.BtConn, a2dpDevName string, hidDevName string) (string, string) {
	var err error
	var a2dpAddr, hidAddr string

	a2dpAddr, err = btConn.GetAddress(a2dpDevName)

	if err != nil {
		s.Log("Failed to get BT address for a2dp device: ", a2dpDevName)
		s.Fatal("MTBF failed: ", err)
	}

	hidAddr, err = btConn.GetAddress(hidDevName)

	if err != nil {
		s.Log("Failed to get BT address for hid device: ", hidDevName)
		s.Fatal("MTBF failed: ", err)
	}

	return a2dpAddr, hidAddr
}

// enableBluetooth enables Bluetooth functionality and checks reconnecting BT devices.
func enableBluetooth(ctx context.Context, s *testing.State, btConn *btconn.BtConn, btDevAddr ...string) {
	s.Log("Try to enable BT")
	err := btConn.SwitchOn()

	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	btConsole, err := btconn.NewBtConsole(ctx, s)

	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	defer btConsole.Close()

	for _, devAddr := range btDevAddr {
		s.Log("Try to reconnect BT device address: ", devAddr)

		if devAddr == notApplicable {
			continue
		}

		if err = btConsole.Connect(devAddr); err != nil {
			s.Fatal("MTBF failed: ", err)
		}
	}
}
