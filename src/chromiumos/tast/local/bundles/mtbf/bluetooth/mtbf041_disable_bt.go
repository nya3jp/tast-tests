// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/bluetooth/btconn"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	a2dpVarName   = "bluetooth.a2dpDevName"
	hidVarName    = "bluetooth.hidDevName"
	notApplicable = "N/A"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF041DisableBT,
		Desc:         "MTBF041 Disable/enable Bluetooth actually disables/enables",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Vars:         []string{"bluetooth.a2dpDevName", "bluetooth.hidDevName"},
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"xliu@cienet.com"},
	})
}

// checkBtNotWorking checks if Bluetooth is working.
func checkBtNotWorking(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	btConsole, mtbferr := btconn.NewBtConsole(ctx, cr)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	defer btConsole.Close(ctx)

	_, mtbferr = btConsole.CheckScanning(ctx, true)

	if strings.Contains(mtbferr.Error(), `Bluetooth (bluez) service not ready`) {
		return
	}

	s.Fatal(mtbferr)
}

// MTBF041DisableBT case verifies the Enable/Disable Bluetooth functionality for A2DP and HID
func MTBF041DisableBT(ctx context.Context, s *testing.State) {
	s.Logf("bt varName: %v, %v", a2dpVarName, hidVarName)
	a2dpDevName := s.RequiredVar(a2dpVarName)
	hidDevName := s.RequiredVar(hidVarName)

	if a2dpDevName == notApplicable {
		s.Fatal(mtbferrors.New(mtbferrors.BTA2DPNeeded, nil, "MTBF041"))
	}

	if hidDevName == notApplicable {
		s.Fatal(mtbferrors.New(mtbferrors.BTHIDNeeded, nil, "MTBF041"))
	}

	cr := s.PreValue().(*chrome.Chrome)
	btConsole, err := btconn.NewBtConsole(ctx, cr)
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}
	defer btConsole.Close(ctx)

	if _, err := btConsole.CheckScanning(ctx, true); err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	btConn := initBluetooth(ctx, s)
	defer btConn.Close(ctx)
	testing.Sleep(ctx, 2*time.Second)
	a2dpDevAddr, hidDevAddr := getBtDeviceAddr(ctx, s, btConn, a2dpDevName, hidDevName)
	s.Logf("a2dp, hid BT device names: %v, %v", a2dpDevName, hidDevName)
	s.Logf("a2dp, hid BT device address: %v, %v", a2dpDevAddr, hidDevAddr)

	if mtbferr := btConn.SwitchOff(ctx); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	checkBtNotWorking(ctx, s)

	s.Log("BT disabled")
	testing.Sleep(ctx, 5*time.Second)
	enableBluetooth(ctx, s, btConn, a2dpDevAddr, hidDevAddr)

	s.Log("BT enabled")
	testing.Sleep(ctx, 5*time.Second)
	btConn.EnterBtPage(ctx)

	if connected, mtbferr := btConn.CheckBtDevice(ctx, a2dpDevName); mtbferr != nil {
		s.Fatal(mtbferr)
	} else if !connected {
		s.Fatal(mtbferrors.New(mtbferrors.BTConnected, nil, a2dpDevName))
	}

	if connected, mtbferr := btConn.CheckBtDevice(ctx, hidDevName); mtbferr != nil {
		s.Fatal(mtbferr)
	} else if !connected {
		s.Fatal(mtbferrors.New(mtbferrors.BTConnected, nil, hidDevName))
	}

	testing.Sleep(ctx, time.Second*10)

	if connected, err := btConn.CheckConnectedByAddr(ctx, a2dpDevAddr); err != nil {
		s.Fatal("MTBF failed: ", err)
	} else if !connected {
		s.Fatal(mtbferrors.New(mtbferrors.BTConnectFailed, nil, a2dpDevAddr))
	}
}

// initBluetooth initializes the Bluetooth connection
func initBluetooth(ctx context.Context, s *testing.State) *btconn.BtConn {
	var cr = s.PreValue().(*chrome.Chrome)
	btConn, mtbferr := btconn.New(ctx, cr, nil)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	return btConn
}

// getBtDeviceAddr retrieves Bt connection addresses for A2DP and HID
func getBtDeviceAddr(ctx context.Context, s *testing.State, btConn *btconn.BtConn, a2dpDevName, hidDevName string) (string, string) {
	var mtbferr error
	var a2dpAddr, hidAddr string

	a2dpAddr, mtbferr = btConn.GetAddress(ctx, a2dpDevName)

	if mtbferr != nil {
		s.Log("Failed to get BT address for a2dp device: ", a2dpDevName)
		s.Fatal(mtbferr)
	}

	hidAddr, mtbferr = btConn.GetAddress(ctx, hidDevName)

	if mtbferr != nil {
		s.Log("Failed to get BT address for hid device: ", hidDevName)
		s.Fatal(mtbferr)
	}

	return a2dpAddr, hidAddr
}

// enableBluetooth enables Bluetooth functionality and checks reconnecting BT devices.
func enableBluetooth(ctx context.Context, s *testing.State, btConn *btconn.BtConn, btDevAddr ...string) {
	s.Log("Try to enable BT")
	mtbferr := btConn.SwitchOn(ctx)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	cr := s.PreValue().(*chrome.Chrome)
	btConsole, mtbferr := btconn.NewBtConsole(ctx, cr)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	defer btConsole.Close(ctx)

	for _, devAddr := range btDevAddr {
		s.Log("Try to reconnect BT device address: ", devAddr)

		if devAddr == notApplicable {
			continue
		}

		if mtbferr = btConsole.Connect(ctx, devAddr); mtbferr != nil {
			s.Fatal(mtbferr)
		}
	}
}
