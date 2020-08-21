// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package btconn

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const termListenerJs = `
	var terminalOutput = '';
	var terminalPid;

	function termLisener(pid, type, text) {
		//console.log(pid, type, text)
		terminalPid = pid;
		terminalOutput += text;
	}

	function isTerminalReady() {
		if (terminalOutput != null && terminalOutput.indexOf('crosh>') > 0) {
			return true;
		}
		return false;
	}

	chrome.terminalPrivate.onProcessOutput.addListener(termLisener);`

const sendInputJs = `chrome.terminalPrivate.sendInput(terminalPid, '%v\n')`

const a2dpInfo = `UUID: Audio Sink                (0000110b-0000-1000-8000-00805f9b34fb)`

const hspInfo = `UUID: Headset                   (00001108-0000-1000-8000-00805f9b34fb)`

// BtConsole represents the CDP connection to a bt_console page.
type BtConsole struct {
	conn *chrome.Conn
}

// NewBtConsole creates a bluetooth console to control bluetooth devices.
//func NewBtConsole(ctx context.Context, s *testing.State) (*BtConsole, error) {
func NewBtConsole(ctx context.Context, cr *chrome.Chrome) (*BtConsole, error) {
	c := &BtConsole{}
	conn, err := cr.NewConn(ctx, `chrome-untrusted://crosh`)

	if err != nil {
		return nil, mtbferrors.New(mtbferrors.OSOpenCrosh, err)
	}

	c.conn = conn

	testing.ContextLog(ctx, "termListenerJs: ", termListenerJs)

	if err := c.conn.Exec(ctx, termListenerJs); err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeTermListener, err)
	}

	if err := conn.WaitForExprWithTimeout(ctx, "isTerminalReady()", 30*time.Second); err != nil {
		return nil, mtbferrors.New(mtbferrors.BTEnterCLI, err)
	}

	testing.ContextLog(ctx, "Entering bt_cosnole")

	if err := c.sendCommand(ctx, "bt_console"); err != nil {
		return nil, mtbferrors.New(mtbferrors.BTEnterCLI, err)
	}

	testing.Sleep(ctx, 2*time.Second)
	return c, nil
}

// Close terminates bluetooth console.
func (c *BtConsole) Close(ctx context.Context) {
	defer c.conn.Close()
	defer c.conn.CloseTarget(ctx)
}

func (c *BtConsole) scanOn(ctx context.Context) error {
	if err := c.sendCommand(ctx, "scan on"); err != nil {
		return mtbferrors.New(mtbferrors.BTCnslCmd, err, "scan on")
	}
	return nil
}

// IsA2dp checks bluetooth device type is A2DP or not.
func (c *BtConsole) IsA2dp(ctx context.Context, btAddress string) (bool, error) {
	info, err := c.GetDeviceInfo(ctx, btAddress)

	if err != nil {
		return false, err
	}

	return strings.Contains(info, a2dpInfo), nil
}

// IsHsp checks bluttooth device type is HSP or not.
func (c *BtConsole) IsHsp(ctx context.Context, btAddress string) (bool, error) {
	info, err := c.GetDeviceInfo(ctx, btAddress)

	if err != nil {
		return false, err
	}

	return strings.Contains(info, hspInfo), nil
}

// GetDeviceInfo gets device information
func (c *BtConsole) GetDeviceInfo(ctx context.Context, address string) (string, error) {
	cmd := fmt.Sprintf("info %v", address)

	if err := c.sendCommand(ctx, cmd); err != nil {
		return "", mtbferrors.New(mtbferrors.BTCnslCmd, err, cmd)
	}

	testing.Sleep(ctx, 1*time.Second)
	return c.getTermTxt(ctx)
}

func (c *BtConsole) scanOff(ctx context.Context) error {
	if err := c.sendCommand(ctx, "scan off"); err != nil {
		return mtbferrors.New(mtbferrors.BTCnslCmd, err, "scan off")
	}

	return nil
}

// Connect connnets to specified bluetooth device.
func (c *BtConsole) Connect(ctx context.Context, btDevAddr string) error {
	retry := 0
	connected := false
	var err error

	for retry < 3 && !connected {
		err = c.connectBtDevice(ctx, btDevAddr)

		if err == nil {
			connected = true
			break
		}

		testing.ContextLogf(ctx, "Failed to connect to BT addr=%v, retry=%v ", btDevAddr, retry)
		retry++
	}

	if connected {
		testing.ContextLog(ctx, "BT device connected! addr: ", btDevAddr)
		return nil
	}
	return err
}

func (c *BtConsole) connectBtDevice(ctx context.Context, btDevAddr string) error {
	cmd := "connect " + btDevAddr

	if err := c.sendCommand(ctx, cmd); err != nil {
		return mtbferrors.New(mtbferrors.BTCnslConn, err, btDevAddr)
	}

	testing.Sleep(ctx, 5*time.Second)

	termTxt, _ := c.getTermTxt(ctx)

	if strings.Contains(termTxt, "org.bluez.Error.Failed") {
		return mtbferrors.New(mtbferrors.BTBluezConnError, nil, btDevAddr)
	}

	if !strings.Contains(termTxt, "Connection successful") {
		return mtbferrors.New(mtbferrors.BTCnslConn, nil, btDevAddr)
	}

	return nil
}

// Disconnect disconnnets to specified bluetooth device.
func (c *BtConsole) Disconnect(ctx context.Context, btDevAddr string) error {
	cmd := "discconnect " + btDevAddr

	if err := c.sendCommand(ctx, cmd); err != nil {
		return mtbferrors.New(mtbferrors.BTCnslCmd, err, cmd)
	}

	return nil
}

func (c *BtConsole) getTermTxt(ctx context.Context) (string, error) {
	var termTxt string
	testing.ContextLog(ctx, "Get terminal text")

	if err := c.conn.Eval(ctx, "terminalOutput", &termTxt); err != nil {
		return "", mtbferrors.New(mtbferrors.ChromeCrosh, err)
	}

	if err := c.conn.Exec(ctx, "terminalOutput = '';"); err != nil {
		return "", mtbferrors.New(mtbferrors.ChromeCrosh, err)
	}

	testing.ContextLog(ctx, "termTxt: ", termTxt)
	return termTxt, nil
}

// CheckScanning starts scanning bluetooth devices.
func (c *BtConsole) CheckScanning(ctx context.Context, on bool) (bool, error) {
	var err error
	err = c.scanOn(ctx)

	if err != nil {
		return false, err
	}

	// TBD timing issue. The message might be printed too fast.
	testing.Sleep(ctx, 2500*time.Millisecond)
	termTxt, err := c.getTermTxt(ctx)

	if err != nil {
		return false, err
	}

	err = c.scanOff(ctx)

	if err != nil {
		return false, err
	}

	if on {
		if strings.Contains(termTxt, "org.bluez.Error.NotReady") {
			return false, mtbferrors.New(mtbferrors.BTServiceNotReady, nil)
		}
		return strings.Contains(termTxt, "Discovery started"), nil
	}
	return strings.Contains(termTxt, "Failed to start discovery"), nil
}

func (c *BtConsole) sendCommand(ctx context.Context, command string) error {
	testing.ContextLog(ctx, "Sleep 2.5 Sec before sending command")
	testing.Sleep(ctx, 2500*time.Millisecond)
	cmdJs := fmt.Sprintf(sendInputJs, command)
	testing.ContextLog(ctx, "cmdJs: ", cmdJs)

	if err := c.conn.Exec(ctx, cmdJs); err != nil {
		return err
	}

	return nil
}
