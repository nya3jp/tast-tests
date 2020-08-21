// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package btconn

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/chrome/settings"
	"chromiumos/tast/testing"
)

const (
	jsTimeout = 30 * time.Second

	settingPage = `document.querySelector("body > os-settings-ui")
		.shadowRoot.querySelector("#main")
		.shadowRoot.querySelector("os-settings-page")`

	btDom = settingPage + `
		.shadowRoot.querySelector("#basicPage > settings-section:nth-child(3) > settings-bluetooth-page")`

	btStatusDom = btDom + `
		.shadowRoot.querySelector("#bluetoothSecondary")`

	clickBtJs = btStatusDom + `.click()`

	enableBtJs = btDom + `
		.shadowRoot.querySelector("#enableBluetooth")
		.shadowRoot.querySelector("#bar").click()`

	btDeviceJs = `Array.from(` + settingPage + `
		.shadowRoot.querySelector("#basicPage > settings-section.expanded > settings-bluetooth-page")
		.shadowRoot.querySelector("#pages > settings-subpage > settings-bluetooth-subpage")
		.shadowRoot.querySelector("#pairedDevices").querySelectorAll("bluetooth-device-list-item"))
		.find(element => element.shadowRoot.querySelector('.list-item > .middle > .name').innerText === '%v')`

	getBtJsTemplate = `
		new Promise(function(resolve, reject) {
			chrome.bluetooth.getDevices(
				function (infos) {
					myDevice = infos.find(device => device.name === %q);

					if (myDevice != null) {
						resolve(%s)
					} else {
						reject(false)
					}
				}
			);
		})
		`

	checkConnectedByAddr = `
		new Promise(function(resolve, reject) {
			chrome.bluetooth.getDevice(%q, function(info) {
				if (info != null) {
					resolve(info.connected);
				} else {
					reject(false);
				}
			});
		})`

	notApplicable = "N/A"
)

// BtConn represends a CDP connection to bluetooth setting page.
type BtConn struct {
	//ctx         context.Context
	cdpConn *chrome.Conn
	//s           *testing.State
	needToClose bool
}

// New creates a object of BtConn.
func New(ctx context.Context, cr *chrome.Chrome, conn *chrome.Conn) (*BtConn, error) {
	c := &BtConn{}
	var err error

	if conn == nil {
		testing.ContextLog(ctx, "conn is nil. Will create new os-setting conn")
		c.cdpConn, err = settings.OpenOsSettingsPage(ctx, cr)

		if err != nil {
			return nil, err
		}

		c.needToClose = true
	} else {
		c.cdpConn = conn
		c.needToClose = false
	}

	if err := c.SwitchOn(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

// Close closes the CDP connection of BT setting page.
func (c *BtConn) Close(ctx context.Context) {
	testing.ContextLog(ctx, "BtConn Close() called. needToClose: ", c.needToClose)

	if c.needToClose {
		defer c.cdpConn.Close()
		defer c.cdpConn.CloseTarget(ctx)
	}
}

// SwitchOn enables ChromeOS bluetooth function.
func (c *BtConn) SwitchOn(ctx context.Context) error {
	testing.ContextLog(ctx, "switchOn - enableBtJs: ", enableBtJs)
	btStatus, err := c.getBtSettingStatus(ctx)

	if err != nil {
		return err
	}

	if btStatus == "On" {
		testing.ContextLog(ctx, "BT is already on. Do nothing")
		return nil
	}

	testing.ContextLog(ctx, "BT is off. Will turn it on")

	if err := c.cdpConn.Exec(ctx, enableBtJs); err != nil {
		return mtbferrors.New(mtbferrors.BTTurnOn, err)
	}

	if err := c.waitForBtStatus(ctx, true); err != nil {
		return mtbferrors.New(mtbferrors.BTTurnOn, err)
	}

	return nil
}

// SwitchOff disables ChromeOS bluetooth function.
func (c *BtConn) SwitchOff(ctx context.Context) error {
	testing.ContextLog(ctx, "SwitchOff - enableBtJs: ", enableBtJs)
	btStatus, err := c.getBtSettingStatus(ctx)

	if err != nil {
		return err
	}

	if btStatus == "Off" {
		testing.ContextLog(ctx, "BT is already off. Do nothing")
		return nil
	}

	testing.ContextLog(ctx, "BT is on. Will turn it off")

	if err := c.cdpConn.Exec(ctx, enableBtJs); err != nil {
		return mtbferrors.New(mtbferrors.BTTurnOff, err)
	}

	if err := c.waitForBtStatus(ctx, false); err != nil {
		return mtbferrors.New(mtbferrors.BTTurnOff, err)
	}

	return nil
}

func (c *BtConn) waitForBtStatus(ctx context.Context, on bool) error {
	targetStatus := "Off"
	if on {
		targetStatus = "On"
	}

	if err := testing.Poll(ctx, func(context.Context) error {
		btStatus, err := c.getBtSettingStatus(ctx)

		if err != nil {
			testing.ContextLog(ctx, "Failed to getBtSettingStatus")
			return err
		}

		if btStatus != targetStatus {
			return errors.New("BT is not " + targetStatus)
		}

		return err
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: jsTimeout}); err != nil {
		testing.ContextLog(ctx, "Polling failed and got error: ", err)
		return err
	}

	return nil
}

func (c *BtConn) getBtSettingStatus(ctx context.Context) (string, error) {
	var btStatus string
	btStatusJs := btStatusDom + ".innerText.trim()"
	testing.ContextLog(ctx, "Get BT setting status js: ", btStatusJs)
	conn := c.cdpConn

	if err := conn.WaitForExprWithTimeout(ctx, btStatusDom+" != null", jsTimeout); err != nil {
		return "", mtbferrors.New(mtbferrors.BTSetting, err)
	}

	conn.Eval(ctx, btStatusJs, &btStatus)
	testing.ContextLog(ctx, "btStatus: ", btStatus)
	return btStatus, nil
}

// EnterBtPage enter bluetooth control page in chrome://os-settings.
func (c *BtConn) EnterBtPage(ctx context.Context) error {
	conn := c.cdpConn
	testing.ContextLog(ctx, "Entering BT setting page")

	if err := conn.Exec(ctx, clickBtJs); err != nil {
		return mtbferrors.New(mtbferrors.BTSetting, err)
	}

	return nil
}

// ClickBtDevice clicks on the BT device.
func (c *BtConn) ClickBtDevice(ctx context.Context, deviceName string) error {
	conn := c.cdpConn
	findBtDeviceJs := fmt.Sprintf(btDeviceJs, deviceName)

	testing.ContextLog(ctx, "findBtDeviceJs: ", findBtDeviceJs)

	if err := conn.WaitForExprWithTimeout(ctx, findBtDeviceJs+" != null", jsTimeout); err != nil {
		return mtbferrors.New(mtbferrors.BTConnect, err, deviceName)
	}

	//Click the device to make it reconnect.
	clickBtDeviceJs := findBtDeviceJs + ".click()"
	testing.ContextLog(ctx, "clickBtDeviceJs: "+clickBtDeviceJs)

	if err := conn.Exec(ctx, clickBtDeviceJs); err != nil {
		testing.ContextLog(ctx, "Failed to Click btDevice: ", err)
		return mtbferrors.New(mtbferrors.BTConnect, err, deviceName)
	}

	return nil
}

// CheckBtDevice check the status of BT device.
func (c *BtConn) CheckBtDevice(ctx context.Context, deviceName string) (bool, error) {
	var connected bool
	testing.Sleep(ctx, 5*time.Second)
	i := 0

	if err := testing.Poll(ctx, func(context.Context) error {
		var err error
		connected, err = c.isDeviceConnected(ctx, deviceName)

		if err != nil || !connected {
			testing.ContextLogf(ctx, "Failed to get BT device i=%d connected=%v btStatus=%v err=%v", i, deviceName, connected, err)
			i++
			return errors.New("BT device is not connected")
		}

		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 30 * time.Second}); err != nil {
		return false, mtbferrors.New(mtbferrors.BTConnect, err, deviceName)
	}

	return connected, nil
}

func (c *BtConn) isDeviceConnected(ctx context.Context, deviceName string) (bool, error) {
	conn := c.cdpConn
	var connected bool

	getBtStatusJs := fmt.Sprintf(getBtJsTemplate, deviceName, "myDevice.connected")
	testing.ContextLog(ctx, "isDeviceConnected - getBtStatusJs: ", getBtStatusJs)

	if err := conn.EvalPromise(ctx, getBtStatusJs, &connected); err != nil {
		return false, mtbferrors.New(mtbferrors.BTGetStatus, err, deviceName)
	}

	testing.ContextLogf(ctx, "deviceName=%v myDevice.connected=%v", deviceName, connected)
	return connected, nil
}

// GetAddress gets the internal address of the BT device.
func (c *BtConn) GetAddress(ctx context.Context, deviceName string) (string, error) {
	if deviceName == notApplicable {
		return notApplicable, nil
	}

	var btAddress string
	getDevAddrJs := fmt.Sprintf(getBtJsTemplate, deviceName, "myDevice.address")
	testing.ContextLog(ctx, "getDevAddrJs: ", getDevAddrJs)

	if err := c.cdpConn.EvalPromise(ctx, getDevAddrJs, &btAddress); err != nil {
		return "", mtbferrors.New(mtbferrors.BTGetAddress, err, deviceName)
	}

	testing.ContextLogf(ctx, "deviceName: %s, btAddress: %s", deviceName, btAddress)
	return btAddress, nil
}

// CheckConnectedByAddr checks if the address is connected.
func (c *BtConn) CheckConnectedByAddr(ctx context.Context, address string) (bool, error) {
	js := fmt.Sprintf(checkConnectedByAddr, address)

	testing.ContextLog(ctx, "CheckConnectedByAddr - js: ", js)

	connected := false

	if err := c.cdpConn.EvalPromise(ctx, js, &connected); err != nil {
		return false, mtbferrors.New(mtbferrors.ChromeExeJs, err, js)
	}

	return connected, nil
}

// CdpConn returns the CDP connection.
func (c *BtConn) CdpConn() *chrome.Conn {
	return c.cdpConn
}
