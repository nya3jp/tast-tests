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

// const btDeviceName = "Magic Mouse"
// const btDeviceName = "Office speaker"
// const btDeviceName = "SonicGear A. P-V"
// const btDeviceName = "Keyboard K370/K375"

const jsTimeout = 30 * time.Second

const settingPage = `document.querySelector("body > os-settings-ui")
	.shadowRoot.querySelector("#main")
	.shadowRoot.querySelector("os-settings-page")`

const btDom = settingPage + `
	.shadowRoot.querySelector("#basicPage > settings-section:nth-child(5) > settings-bluetooth-page")`

const btStatusDom = btDom + `
	.shadowRoot.querySelector("#bluetoothSecondary")`

const clickBtJs = btStatusDom + `.click()`

const enableBtJs = btDom + `
	.shadowRoot.querySelector("#enableBluetooth")
	.shadowRoot.querySelector("#bar").click()`

const btDeviceJs = `Array.from(` + settingPage + `
	.shadowRoot.querySelector("#basicPage > settings-section.expanded > settings-bluetooth-page")
	.shadowRoot.querySelector("#pages > settings-subpage > settings-bluetooth-subpage")
	.shadowRoot.querySelector("#pairedDevices").querySelectorAll("bluetooth-device-list-item"))
	.find(element => element.shadowRoot.querySelector('.list-item > .middle > .name').innerText === '%v')`

const getBtDeviceJs = `
		var myDevice = null;

		function isMyDevice(device) {
			return device.name === "%v";
		}

		chrome.bluetooth.getDevices(
			function (infos) {
				myDevice = infos.find(isMyDevice);
			}
		)
		`

const btCleanUpJs = `myDevice = null;`

const notApplicable = "N/A"

// BtConn represends a CDP connection to bluetooth setting page.
type BtConn struct {
	ctx         context.Context
	cdpConn     *chrome.Conn
	s           *testing.State
	needToClose bool
}

// New creates a object of BtConn.
func New(ctx context.Context, s *testing.State, cr *chrome.Chrome, conn *chrome.Conn) (*BtConn, error) {
	c := &BtConn{ctx: ctx, s: s}
	var err error

	if conn == nil {
		s.Log("conn is nil. Will create new os-setting conn")
		cr := s.PreValue().(*chrome.Chrome)
		c.cdpConn, err = settings.OpenOsSettingsPage(ctx, cr)

		if err != nil {
			return nil, err
		}

		c.needToClose = true
	} else {
		c.cdpConn = conn
		c.needToClose = false
	}

	if err := c.SwitchOn(); err != nil {
		return nil, err
	}

	return c, nil
}

// Close closes the CDP connection of BT setting page.
func (c *BtConn) Close() {
	c.s.Log("BtConn Close() called. needToClose: ", c.needToClose)

	if c.needToClose {
		defer c.cdpConn.Close()
		defer c.cdpConn.CloseTarget(c.ctx)
	}
}

// SwitchOn enables ChromeOS bluetooth function.
func (c *BtConn) SwitchOn() error {
	c.s.Log("switchOn - enableBtJs: ", enableBtJs)
	btStatus, err := c.getBtSettingStatus()

	if err != nil {
		return err
	}

	if btStatus == "On" {
		c.s.Log("BT is already on. Do nothing.")
		return nil
	}

	c.s.Log("BT is off. Will turn it on.")

	if err := c.cdpConn.Exec(c.ctx, enableBtJs); err != nil {
		return mtbferrors.New(mtbferrors.BTTurnOn, err)
	}

	if err := c.waitForBtStatus(true); err != nil {
		return mtbferrors.New(mtbferrors.BTTurnOn, err)
	}

	return nil
}

// SwitchOff disables ChromeOS bluetooth function.
func (c *BtConn) SwitchOff() error {
	c.s.Log("SwitchOff - enableBtJs: ", enableBtJs)
	btStatus, err := c.getBtSettingStatus()

	if err != nil {
		return err
	}

	if btStatus == "Off" {
		c.s.Log("BT is already off. Do nothing.")
		return nil
	}

	c.s.Log("BT is on. Will turn it off.")

	if err := c.cdpConn.Exec(c.ctx, enableBtJs); err != nil {
		return mtbferrors.New(mtbferrors.BTTurnOff, err)
	}

	if err := c.waitForBtStatus(false); err != nil {
		return mtbferrors.New(mtbferrors.BTTurnOff, err)
	}

	return nil
}

func (c *BtConn) waitForBtStatus(on bool) error {
	var targetStatus string

	if on {
		targetStatus = "On"
	} else {
		targetStatus = "Off"
	}

	if err := testing.Poll(c.ctx, func(context.Context) error {
		btStatus, err := c.getBtSettingStatus()

		if err != nil {
			c.s.Log("Failed to getBtSettingStatus")
			return err
		}

		if btStatus != targetStatus {
			return errors.New("BT is not " + targetStatus)
		}

		return err
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: jsTimeout}); err != nil {
		c.s.Log("Polling failed and got error: ", err)
		return err
	}

	return nil
}

func (c *BtConn) getBtSettingStatus() (string, error) {
	var btStatus string
	btStatusJs := btStatusDom + ".innerText.trim()"
	c.s.Log("Get BT setting status js: ", btStatusJs)
	conn := c.cdpConn
	ctx := c.ctx

	if err := conn.WaitForExprWithTimeout(ctx, btStatusDom+" != null", jsTimeout); err != nil {
		return "", mtbferrors.New(mtbferrors.BTSetting, err)
	}

	conn.Eval(ctx, btStatusJs, &btStatus)
	c.s.Log("btStatus: ", btStatus)
	return btStatus, nil
}

// EnterBtPage enter bluetooth control page in chrome://os-settings.
func (c *BtConn) EnterBtPage() error {
	s := c.s
	conn := c.cdpConn
	ctx := c.ctx
	s.Log("Entering BT setting page")

	if err := conn.Exec(ctx, clickBtJs); err != nil {
		return mtbferrors.New(mtbferrors.BTSetting, err)
	}

	return nil
}

// ClickBtDevice clicks on the BT device.
func (c *BtConn) ClickBtDevice(deviceName string) error {
	conn := c.cdpConn
	s := c.s
	ctx := c.ctx
	findBtDeviceJs := fmt.Sprintf(btDeviceJs, deviceName)

	s.Log("findBtDeviceJs: ", findBtDeviceJs)

	if err := conn.WaitForExprWithTimeout(ctx, findBtDeviceJs+" != null", jsTimeout); err != nil {
		return mtbferrors.New(mtbferrors.BTConnect, err, deviceName)
	}

	//Click the device to make it reconnect.
	clickBtDeviceJs := findBtDeviceJs + ".click()"
	s.Log("clickBtDeviceJs: " + clickBtDeviceJs)

	if err := conn.Exec(ctx, clickBtDeviceJs); err != nil {
		s.Log("Failed to Click btDevice: ", err)
		return mtbferrors.New(mtbferrors.BTConnect, err, deviceName)
	}

	return nil
}

// CheckBtDevice check the status of BT device.
func (c *BtConn) CheckBtDevice(deviceName string) (bool, error) {
	ctx := c.ctx
	testing.Sleep(ctx, 5*time.Second)

	var btStatus string

	if err := testing.Poll(ctx, func(context.Context) error {
		var err error
		btStatus, err = c.getBtStatus(deviceName)

		if btStatus != "true" {
			return errors.New("BT device is not connected")
		}

		return err
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 30 * time.Second}); err != nil {
		return false, mtbferrors.New(mtbferrors.BTConnect, err, deviceName)
	}

	return (btStatus == "true"), nil
}

func (c *BtConn) runGetDeviceJs(deviceName string) error {
	js := fmt.Sprintf(getBtDeviceJs, deviceName)
	c.s.Log("js: ", js)

	if err := c.cdpConn.Exec(c.ctx, js); err != nil {
		c.s.Log("Failed to get BT status: ", err)
		return mtbferrors.New(mtbferrors.BTGetStatus, err, deviceName)
	}

	if err := c.cdpConn.WaitForExprWithTimeout(c.ctx, "myDevice != null", jsTimeout); err != nil {
		c.s.Log("Failed to get myDevice status ", err)
		return mtbferrors.New(mtbferrors.BTGetStatus, err, deviceName)
	}

	return nil
}

func (c *BtConn) getBtStatus(deviceName string) (string, error) {
	conn := c.cdpConn
	s := c.s
	ctx := c.ctx
	defer c.cdpConn.Exec(c.ctx, btCleanUpJs)

	if err := c.runGetDeviceJs(deviceName); err != nil {
		return "false", mtbferrors.New(mtbferrors.BTGetStatus, err, deviceName)
	}

	if err := conn.WaitForExprWithTimeout(ctx, "myDevice != null && myDevice.connected != null", jsTimeout); err != nil {
		s.Log("Failed to get myDevice status: ", err)
		return "false", mtbferrors.New(mtbferrors.BTGetStatus, err, deviceName)
	}

	var btStatus string

	// conn.Eval only works for string!
	conn.Eval(ctx, "myDevice.connected + ''", &btStatus)
	s.Log("myDevice.connected: ", btStatus)
	return btStatus, nil
}

// GetAddress gets the internal address of the BT device.
func (c *BtConn) GetAddress(deviceName string) (string, error) {
	if deviceName == notApplicable {
		return notApplicable, nil
	}

	var btAddress string
	defer c.cdpConn.Exec(c.ctx, btCleanUpJs)

	if err := c.runGetDeviceJs(deviceName); err != nil {
		return "", mtbferrors.New(mtbferrors.BTGetStatus, err, deviceName)
	}

	c.cdpConn.Eval(c.ctx, "myDevice.address", &btAddress)
	c.s.Logf("deviceName: %v, btAddress: %v", deviceName, btAddress)
	return btAddress, nil
}

// CdpConn returns the CDP connection.
func (c *BtConn) CdpConn() *chrome.Conn {
	return c.cdpConn
}
