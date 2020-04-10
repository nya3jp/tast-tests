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
	"chromiumos/tast/local/input"
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

	chrome.terminalPrivate.onProcessOutput.addListener(termLisener);`

const sendInputJs = `chrome.terminalPrivate.sendInput(terminalPid, '%v\n')`

const a2dpInfo = `UUID: Audio Sink                (0000110b-0000-1000-8000-00805f9b34fb)`

const hspInfo = `UUID: Headset                   (00001108-0000-1000-8000-00805f9b34fb)`

// BtConsole represents the CDP connection to a bt_console page.
type BtConsole struct {
	ctx   context.Context
	conn  *chrome.Conn
	tconn *chrome.Conn
	s     *testing.State
	kb    *input.KeyboardEventWriter
}

// NewBtConsole creates a bluetooth console to control bluetooth devices.
func NewBtConsole(ctx context.Context, s *testing.State) (*BtConsole, error) {
	c := &BtConsole{ctx: ctx, s: s}
	cr := s.PreValue().(*chrome.Chrome)
	conn, err := cr.NewConn(ctx, `chrome-extension://nkoccljplnhpfnfiajclkommnmllphnl/html/crosh.html`)

	if err != nil {
		return nil, mtbferrors.New(mtbferrors.OSOpenCrosh, err)
	}

	tconn, err := cr.TestAPIConn(ctx)

	if err != nil {
		return nil, mtbferrors.New(mtbferrors.OSOpenCrosh, err)
	}

	c.conn = conn
	c.tconn = tconn

	kb, err := input.Keyboard(c.ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeExeJs, err, "input.Keyboard")
	}

	c.kb = kb

	s.Log("termListenerJs: ", termListenerJs)

	if err := c.conn.Exec(c.ctx, termListenerJs); err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeTermListener, err)
	}

	testing.Sleep(ctx, 5*time.Second)
	s.Log("Entering bt_cosnole by keyboard")

	if err := c.kb.Type(c.ctx, "bt_console\n"); err != nil {
		return nil, mtbferrors.New(mtbferrors.BTEnterCLI, err)
	}

	testing.Sleep(c.ctx, 2*time.Second)
	return c, nil
}

// Close terminates bluetooth console.
func (c *BtConsole) Close() {
	defer c.tconn.Close()
	defer c.tconn.CloseTarget(c.ctx)
	defer c.conn.Close()
	defer c.conn.CloseTarget(c.ctx)
	defer c.kb.Close()
}

func (c *BtConsole) scanOn() error {
	if err := c.sendCommand("scan on"); err != nil {
		return mtbferrors.New(mtbferrors.BTCnslCmd, err, "scan on")
	}

	return nil
}

// IsA2dp checks bluetooth device type is A2DP or not.
func (c *BtConsole) IsA2dp(btAddress string) (bool, error) {
	info, err := c.GetDeviceInfo(btAddress)

	if err != nil {
		return false, err
	}

	return strings.Contains(info, a2dpInfo), nil
}

// IsHsp checks bluttooth device type is HSP or not.
func (c *BtConsole) IsHsp(btAddress string) (bool, error) {
	info, err := c.GetDeviceInfo(btAddress)

	if err != nil {
		return false, err
	}

	return strings.Contains(info, hspInfo), nil
}

// GetDeviceInfo gets device information
func (c *BtConsole) GetDeviceInfo(address string) (string, error) {
	cmd := fmt.Sprintf("info %v", address)

	if err := c.sendCommand(cmd); err != nil {
		return "", mtbferrors.New(mtbferrors.BTCnslCmd, err, cmd)
	}

	testing.Sleep(c.ctx, 1*time.Second)
	return c.getTermTxt()
}

// IsConnected checks if the address is connected.
func (c *BtConsole) IsConnected(address string) (bool, error) {
	js := fmt.Sprintf(`
		new Promise(function(resolve, reject) {
			chrome.bluetooth.getDevice(%q, function(info) {
				if (info != null) {
					resolve(info.connected);
				} else {
					reject(false);
				}
			});
		})
	`, address)

	connected := false
	if err := c.tconn.EvalPromise(c.ctx, js, &connected); err != nil {
		return false, mtbferrors.New(mtbferrors.ChromeExeJs, err, js)
	}
	return connected, nil
}

func (c *BtConsole) scanOff() error {
	if err := c.sendCommand("scan off"); err != nil {
		return mtbferrors.New(mtbferrors.BTCnslCmd, err, "scan off")
	}

	return nil
}

// Connect connnets to specified bluetooth device.
func (c *BtConsole) Connect(btDevAddr string) error {
	retry := 0
	connected := false
	var err error

	for retry < 3 && !connected {
		err = c.connectBtDevice(btDevAddr)

		if err == nil {
			connected = true
			break
		}

		c.s.Logf("Failed to connect to BT addr=%v, retry=%v ", btDevAddr, retry)
		retry++
	}

	if connected {
		c.s.Log("BT device connected! addr: ", btDevAddr)
		return nil
	}
	return err
}

func (c *BtConsole) connectBtDevice(btDevAddr string) error {
	cmd := "connect " + btDevAddr

	if err := c.sendCommand(cmd); err != nil {
		return mtbferrors.New(mtbferrors.BTCnslConn, err, btDevAddr)
	}

	testing.Sleep(c.ctx, 5*time.Second)

	termTxt, _ := c.getTermTxt()

	if strings.Contains(termTxt, "org.bluez.Error.Failed") {
		return mtbferrors.New(mtbferrors.BTBluezConnError, nil, btDevAddr)
	}

	if !strings.Contains(termTxt, "Connection successful") {
		return mtbferrors.New(mtbferrors.BTCnslConn, nil, btDevAddr)
	}

	return nil
}

// Disconnect disconnnets to specified bluetooth device.
func (c *BtConsole) Disconnect(btDevAddr string) error {
	cmd := "discconnect " + btDevAddr

	if err := c.sendCommand(cmd); err != nil {
		return mtbferrors.New(mtbferrors.BTCnslCmd, err, cmd)
	}

	return nil
}

func (c *BtConsole) getTermTxt() (string, error) {
	var termTxt string
	c.s.Log("Get terminal text...")

	if err := c.conn.Eval(c.ctx, "terminalOutput", &termTxt); err != nil {
		return "", mtbferrors.New(mtbferrors.ChromeCrosh, err)
	}

	if err := c.conn.Exec(c.ctx, "terminalOutput = '';"); err != nil {
		return "", mtbferrors.New(mtbferrors.ChromeCrosh, err)
	}

	c.s.Log("termTxt: ", termTxt)
	return termTxt, nil
}

// CheckScanning starts scanning bluetooth devices.
func (c *BtConsole) CheckScanning(on bool) (bool, error) {
	var err error
	err = c.scanOn()

	if err != nil {
		return false, err
	}

	// TBD timing issue. The message might be printed too fast.
	testing.Sleep(c.ctx, 2500*time.Millisecond)
	termTxt, err := c.getTermTxt()

	if err != nil {
		return false, err
	}

	err = c.scanOff()

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

func (c *BtConsole) sendCommand(command string) error {
	c.s.Log("Sleep 2.5 Sec before sending command")
	testing.Sleep(c.ctx, 2500*time.Millisecond)
	cmdJs := fmt.Sprintf(sendInputJs, command)
	c.s.Log("cmdJs: ", cmdJs)

	if err := c.conn.Exec(c.ctx, cmdJs); err != nil {
		return err
	}

	return nil
}
