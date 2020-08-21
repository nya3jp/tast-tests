// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/bluetooth/btconn"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF040BTWifiCoexist,
		Desc:         "MTBF040 BT / Wifi coexist test",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Vars:         []string{"bluetooth.a2dpDevName", "bluetooth.hidDevName"},
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"xliu@cienet.com"},
		Timeout:      5 * time.Minute,
	})
}

// MTBF040BTWifiCoexist case test different combinations of enabling/disabling the Bluetooth and wifi.
func MTBF040BTWifiCoexist(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "mtbf040_bt_wifi_coexist")
	defer st.End()
	var btConn *btconn.BtConn
	cr := s.PreValue().(*chrome.Chrome)
	a2dpDevName := s.RequiredVar("bluetooth.a2dpDevName")
	hidDevName := s.RequiredVar("bluetooth.hidDevName")
	s.Logf("MTBF040 a2dp, hid BT device names: %v, %v", a2dpDevName, hidDevName)

	wifiConn, mtbferr := wifi.NewConn(ctx, cr, true, "", "", "", "")
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer wifiConn.Close(true)

	btConn, mtbferr = btconn.New(ctx, cr, wifiConn.CdpConn())
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer btConn.Close(ctx)

	a2dpDevAddr, hidDevAddr := getDeviceAddr(ctx, s, btConn, a2dpDevName, hidDevName)

	s.Log("Scenario 1 BT ON and WiFiOn - 1.1 Check both BT and WiFi scanning working")
	checkBtWorking(ctx, s)
	checkWifiWorking(ctx, s, wifiConn)
	s.Log("1.2 Now turn off WiFi")
	disableWifi(s, wifiConn)
	s.Log("1.2 Verification: Make sure BT is on and device scan works")
	checkBtWorking(ctx, s)

	s.Log("Scenario 2 BT ON and WiFiOn")
	enableWifi(s, wifiConn)
	s.Log("2.1 Check if both BT and WiFi scanning working")
	checkBtWorking(ctx, s)
	checkWifiWorking(ctx, s, wifiConn)
	s.Log("2.2 Now turn off BT")
	disableBt(ctx, s, btConn)
	s.Log("2.2 Verification: Make sure WiFi is On and network scan works")
	checkWifiWorking(ctx, s, wifiConn)

	s.Log("Scenario 3: BT On, Wi-Fi Off")
	enableBt(ctx, s, btConn, a2dpDevAddr, hidDevAddr)
	disableWifi(s, wifiConn)
	s.Log("3.1 Check if BT scanning works")
	checkBtWorking(ctx, s)
	s.Log("3.2 Now turn off BT")
	disableBt(ctx, s, btConn)
	s.Log("3.3 Turn on Wi-Fi")
	enableWifi(s, wifiConn)
	s.Log("3.3 Verification: Make sure we can turn on Wi-fi and network scan works")
	checkWifiWorking(ctx, s, wifiConn)

	s.Log("Scenario 4: BT Off, Wi-Fi On") // BT is already Off.
	s.Log("4.1 Check if Wi-Fi scanning works")
	checkWifiWorking(ctx, s, wifiConn)
	s.Log("4.2 Now turn off Wi-Fi")
	disableWifi(s, wifiConn)
	s.Log("4.3 Turn on BT")
	enableBt(ctx, s, btConn, a2dpDevAddr, hidDevAddr)
	s.Log("4.3 Verification: Make sure we can turn on BT and device scan works")
	checkBtWorking(ctx, s)

	testing.Sleep(ctx, time.Second*10)

	btConsole, err := btconn.NewBtConsole(ctx, cr)
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}
	defer btConsole.Close(ctx)

	if connected, err := btConn.CheckConnectedByAddr(ctx, a2dpDevAddr); err != nil {
		s.Fatal("MTBF failed: ", err)
	} else if !connected {
		s.Fatal(mtbferrors.New(mtbferrors.BTConnectFailed, nil, a2dpDevAddr))
	}
}

// disableBt disables Bluetooth functionality.
func disableBt(ctx context.Context, s *testing.State, btConn *btconn.BtConn) {
	s.Log("Try to disable BT")
	mtbferr := btConn.SwitchOff(ctx)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
}

// enableBt enables Bluetooth functionality and checks reconnecting BT devices.
func enableBt(ctx context.Context, s *testing.State, btConn *btconn.BtConn, btDevAddr ...string) {
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

	notApplicable := "N/A"
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

// disableWifi disables Wifi.
func disableWifi(s *testing.State, wifiConn *wifi.Conn) {
	s.Log("Try to disable WiFi")
	wifiStatus, mtbferr := wifiConn.DisableWifi()

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Wifi status after disabled: ", wifiStatus)
}

// enableWifi enables Wifi.
func enableWifi(s *testing.State, wifiConn *wifi.Conn) {
	s.Log("Try to enable WiFi")
	wifiStatus, mtbferr := wifiConn.EnableWifi()

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Wifi status after enabled: ", wifiStatus)
}

// getDeviceAddr retrieves Bt connection addresses for A2DP and HID.
func getDeviceAddr(ctx context.Context, s *testing.State, btConn *btconn.BtConn, a2dpDevName, hidDevName string) (string, string) {
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

// checkBtWorking checks if Bluetooth is working.
func checkBtWorking(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	btConsole, mtbferr := btconn.NewBtConsole(ctx, cr)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	defer btConsole.Close(ctx)

	if scanning, mtbferr := btConsole.CheckScanning(ctx, true); mtbferr != nil {
		s.Fatal(mtbferr)
	} else if !scanning {
		s.Fatal(mtbferrors.New(mtbferrors.BTScan, nil))
	}
}

// checkWifiWorking checks if Wifi is working.
func checkWifiWorking(ctx context.Context, s *testing.State, wifiConn *wifi.Conn) {
	var wifiListOk bool
	mtbferr := wifiConn.EnterWifiPage()

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	wifiListOk, mtbferr = wifiConn.CheckWifiListDisplayed()

	if mtbferr != nil {
		s.Fatal(mtbferr)
	} else if !wifiListOk {
		s.Fatal(mtbferrors.New(mtbferrors.WIFIAPlist, nil))
	}

	mtbferr = wifiConn.LeaveWifiPage()

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("WiFi is working")
}
