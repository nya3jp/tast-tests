// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cisco

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// Controller is the handle object for wrapper of the Cisco wireless controller device.
type Controller struct {
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stdoutScanner *bufio.Scanner
	cmd           *ssh.Cmd
	commands      chan string
	wlans         map[int](*wlanConfig)
	aps           map[string](*apData)
}

const (
	promptText   = "(Cisco Controller)"
	confirmText  = "(y/n) "
	confirm2Text = "(y/N)"
	moreText     = "--More-- or (q)uit"

	defaultGroupName = "default-group"

	loginTimeout = 10 * time.Second
	cmdTimeout   = 20 * time.Second
)

var (
	matcherNumberWLANs = regexp.MustCompile(`Number of WLANs\.+ (\d+)`)
	matcherWLANRow     = regexp.MustCompile(`(\d+)\s+(\S+) / (\S+)\s+(\S+)\s+(\S+)\s+(\S+)`)
	matcherAPGroupName = regexp.MustCompile(`Site Name\.+ (\S+)`)
	matcherAPRow       = regexp.MustCompile(`(\S+)\s+(\d+)\s+(\S+)\s+(\S+)`)
	matcherNumberAPs   = regexp.MustCompile(`Number of APs\.+ (\d+)`)
	matcherNetwork     = regexp.MustCompile(`Network\.+ (\d+).(\d+).(\d+).(\d+)`)
	matcherNetmask     = regexp.MustCompile(`Netmask\.+ (\d+).(\d+).(\d+).(\d+)`)

	apOnlinePollOptions = testing.PollOptions{
		Timeout:  5 * time.Minute,
		Interval: 10 * time.Second,
	}
)

type wlanConfig struct {
	id        int
	ssid      string
	networkIP net.IP
	netmask   net.IPMask
}

type apData struct {
	name string
	ap   *AccessPoint
}

// InitCiscoController creates Controller, connects and logs in to the controller device.
func InitCiscoController(ctx context.Context, proxyConn *ssh.Conn, hostname, user, password string) (*Controller, error) {
	ctx, st := timing.Start(ctx, "ConnectCiscoController")
	defer st.End()

	var ctrl Controller

	err := ctrl.openConnection(ctx, hostname, proxyConn)
	if err != nil {
		return nil, errors.Wrapf(err, "could not open connection to Cisco controller %s", hostname)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	// wait for initial prompt line
	if _, err := ctrl.waitCompleteResult(timeoutCtx, true); err != nil {
		return nil, errors.Wrap(err, "failed waiting for Cisco controller login prompt")
	}
	if _, err := io.WriteString(ctrl.stdin, user+"\n"); err != nil {
		return nil, errors.Wrap(err, "failed to send login to Cisco controller")
	}
	if _, err := io.WriteString(ctrl.stdin, password+"\n"); err != nil {
		return nil, errors.Wrap(err, "failed to send password to Cisco controller")
	}

	if _, err := ctrl.waitCompleteResult(timeoutCtx, true); err != nil {
		return nil, errors.Wrap(err, "failed waiting for Cisco controller command prompt")
	}

	ctrl.initAPs(ctx)

	if len(ctrl.aps) < 1 {
		return nil, errors.New("no Cisco APs available")
	}

	if err := ctrl.deleteAllWLANs(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to delete WLANs")
	}

	testing.ContextLog(ctx, "WLANs deleted")

	return &ctrl, nil
}

// Close logs out of controller CLI and closes all resources used.
func (ctrl *Controller) Close(ctx context.Context) error {
	// for _, wlan := range ctrl.wlans {
	// 	if err := ctrl.deleteWLAN(ctx, wlan.id); err != nil {
	// 		return errors.Wrap(err, "failed to delete WLAN")
	// 	}
	// }

	// asnwer 'no' to question on configuration save
	_, err := ctrl.sendCommand(ctx, "logout\n", false)
	if err != nil {
		return errors.Wrap(err, "failed to logout")
	}

	testing.ContextLog(ctx, "Successful logout from controller CLI")

	return ctrl.closeConnection(ctx)
}

func (ctrl *Controller) openConnection(ctx context.Context, hostname string, proxyConn *ssh.Conn) error {
	cmd := proxyConn.Command("sudo", "ssh", hostname)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdin pipe to controller console")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdout pipe from controller console")
	}

	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to ssh to controller")
	}

	ctrl.commands = make(chan string, 100)

	ctrl.stdin = stdin
	ctrl.stdout = stdout
	ctrl.stdoutScanner = bufio.NewScanner(stdout)
	ctrl.stdoutScanner.Split(scanPrompt)
	ctrl.cmd = cmd
	ctrl.wlans = make(map[int](*wlanConfig))

	go func() {
		defer close(ctrl.commands)
		for ctrl.stdoutScanner.Scan() {
			out := ctrl.stdoutScanner.Text()
			ctrl.commands <- out
		}
	}()

	return nil
}

func (ctrl *Controller) closeConnection(ctx context.Context) error {

	if err := ctrl.stdin.Close(); err != nil {
		return errors.Wrap(err, "failed to close controller CLI stdin")
	}

	if err := ctrl.cmd.Wait(ctx); err != nil {
		// do not return as error, on success ssh exits with code 1 which
		// is returned from Wait as an error
		testing.ContextLog(ctx, "Closed Cisco controller CLI session: "+err.Error())
	}

	// drain ctrl.commands in case scan goroutine is stuck on writing to a full channel
	// and wait until it's closed there
	for range ctrl.commands {
	}

	ctrl.stdoutScanner = nil
	if err := ctrl.stdout.Close(); err != nil {
		return errors.Wrap(err, "failed to close controller CLI stdout")
	}

	return nil
}

// scanPrompt is a split function for a Scanner that returns each command
// with its result.
// Command prompt text "(Cisco Controller)" is used as a separator of commands,
// it is stripped from the result. The prompt char ">" is left to indicate
// start of command.
// The last non-empty line of input will be returned even if it does not end
// with command prompt.
func scanPrompt(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, []byte(promptText)); i >= 0 {
		// advance over the prompt and trailing spaces
		skip := len(promptText)
		for data[i+skip] == ' ' && i+skip < len(data) {
			skip++
		}
		// We have a full prompt-terminated buffer
		return i + skip, data[0:i], nil
	}
	if i := bytes.Index(data, []byte(confirmText)); i >= 0 {
		skip := len(confirmText)
		return i + skip, data[0 : i+skip], nil
	}
	if i := bytes.Index(data, []byte(confirm2Text)); i >= 0 {
		skip := len(confirm2Text)
		return i + skip, data[0 : i+skip], nil
	}
	if i := bytes.Index(data, []byte(moreText)); i >= 0 {
		skip := len(moreText)
		return i + skip, data[0 : i+skip], nil
	}
	// If we're at EOF, we have a final, non-terminated result. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

// waitCompleteResult waits for complete command result to appear in stdout of controller CLI
// (until next command prompt - see function scanPrompt)
// and returns the result stripped of command text itself.
func (ctrl *Controller) waitCompleteResult(ctx context.Context, acceptOnQuestion bool) (result string, err error) {
	partsText := ""
	for {
		select {
		case <-ctx.Done():
			testing.ContextLog(ctx, "timeout")
			return "", errors.New("Timeout while waiting for prompt")
		case cmd := <-ctrl.commands:
			if len(cmd) == 0 {
				continue
			}
			//testing.ContextLog(ctx, "cmd: ", cmd)
			if strings.HasSuffix(cmd, confirmText) || strings.HasSuffix(cmd, confirm2Text) {
				reply := "n"
				if acceptOnQuestion {
					reply = "y"
				}
				if _, err := io.WriteString(ctrl.stdin, reply); err != nil {
					return "", errors.Wrap(err, "failed to send 'y' to Cisco controller")
				}
				continue
			}
			if strings.HasSuffix(cmd, moreText) {
				partsText = partsText + cmd[0:len(cmd)-len(moreText)]
				if _, err := io.WriteString(ctrl.stdin, " "); err != nil {
					return "", errors.Wrap(err, "failed to send ' ' to Cisco controller")
				}
				continue
			}
			if cmd[0] == '>' {
				// remove leading prompt and command
				newLineIdx := strings.IndexRune(cmd, '\n')
				if newLineIdx != -1 {
					return cmd[newLineIdx+1:], nil
				}
			}
			return partsText + cmd, nil
		}
	}
	testing.ContextLog(ctx, "ret?")
	return "", nil
}

// sendCommand executes a command in controller CLI and waits for its complete result.
// (see waitCompleteResult)
func (ctrl *Controller) sendCommand(ctx context.Context, cmd string, acceptOnQuestion bool) (output string, err error) {
	testing.ContextLog(ctx, "cmd: ", cmd)

	timeoutCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
	defer cancel()
	if _, err := io.WriteString(ctrl.stdin, cmd+"\n"); err != nil {
		testing.ContextLog(ctx, "Failed to send command to Cisco controller: ", err)
	}

	return ctrl.waitCompleteResult(timeoutCtx, acceptOnQuestion)
}

func (ctrl *Controller) sendCommandNoResult(ctx context.Context, cmd string) (err error) {
	result, err := ctrl.sendCommand(ctx, cmd, true)

	if err != nil {
		return err
	}

	result = strings.TrimSpace(result)
	if result != "" {
		return errors.New("Unexpected command output: " + result)
	}

	return err
}

func (ctrl *Controller) getWLANs(ctx context.Context) ([]*wlanConfig, error) {
	out, err := ctrl.sendCommand(ctx, "show wlan summary", true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get wlan summary")
	}

	lines := strings.Split(out, "\n")

	numberWLANs := 0
	firstRowIdx := 0
	for idx, line := range lines {
		if matches := matcherNumberWLANs.FindStringSubmatch(line); matches != nil {
			numberWLANs, err = strconv.Atoi(matches[1])
			if err != nil {
				return nil, errors.Wrap(err, "error parsing number of WLANs: "+matches[1])
			}
		} else if strings.HasPrefix(line, "WLAN ID") {
			firstRowIdx = idx + 2 // skip separator line
		}
	}

	wlans := make([]*wlanConfig, numberWLANs)
	for idx, line := range lines[firstRowIdx : firstRowIdx+numberWLANs] {
		if matches := matcherWLANRow.FindStringSubmatch(line); matches != nil {
			var wlan wlanConfig
			wlan.id, err = strconv.Atoi(matches[1])
			if err != nil {
				return nil, errors.Wrap(err, "error parsing number WLAN id: "+matches[1])
			}
			wlan.ssid = matches[3]
			wlans[idx] = &wlan
		}
	}

	return wlans, nil
}

func (ctrl *Controller) deleteWLAN(ctx context.Context, id int) error {
	testing.ContextLog(ctx, "deleting WLAN: ", id)

	_, err := ctrl.sendCommand(ctx, fmt.Sprintf("config wlan delete %d", id), true)
	if err != nil {
		return errors.Wrap(err, "failed to delete WLAN")
	}

	return nil
}

func (ctrl *Controller) deleteAllWLANs(ctx context.Context) error {
	wlans, err := ctrl.getWLANs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get WLANs")
	}

	for _, wlan := range wlans {
		err := ctrl.deleteWLAN(ctx, wlan.id)
		if err != nil {
			return errors.Wrap(err, "failed to delete WLAN")
		}
	}

	return nil
}

func (ctrl *Controller) findWLAN(ctx context.Context, ssid string) (wlan *wlanConfig, err error) {
	wlans, err := ctrl.getWLANs(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get WLANs")
	}

	for _, wlan := range wlans {
		if wlan.ssid == ssid {
			return wlan, nil
		}
	}

	return nil, nil
}

func (ctrl *Controller) findUnusedWLANId(ctx context.Context) (id int, err error) {
	wlans, err := ctrl.getWLANs(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get WLANs")
	}

	id = 1
iterateIds:
	for {
		for _, wlan := range wlans {
			if wlan.id == id {
				continue iterateIds
			}
		}
		return id, nil
	}
}

// createWLAN sets up WLAN in the controller.
// if config.id == 0 then id is generated automatically and written to config on function return
// otherwise the provided id is used, it should not conflict with ids of existing networks.
func (ctrl *Controller) createWLAN(ctx context.Context, config *wlanConfig) error {
	if config.id == 0 {
		var err error
		config.id, err = ctrl.findUnusedWLANId(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to find wlan id")
		}
	}

	err := ctrl.sendCommandNoResult(ctx, fmt.Sprintf("config wlan create %d %s", config.id, config.ssid))
	if err != nil {
		return errors.Wrap(err, "failed to create wlan")
	}

	ctrl.wlans[config.id] = config

	err = ctrl.sendCommandNoResult(ctx, fmt.Sprintf("config wlan security wpa disable %d", config.id))
	if err != nil {
		return errors.Wrap(err, "failed to create wlan")
	}

	out, err := ctrl.sendCommand(ctx, "show dhcp detailed day0-dhcp-mgmt", true)
	if err != nil {
		return errors.Wrap(err, "failed to get DHCP config")
	}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if matches := matcherNetwork.FindStringSubmatch(line); matches != nil {
			config.networkIP = net.IPv4(0, 0, 0, 0)
			for i := 0; i < 4; i++ {
				ipByte, err := strconv.Atoi(matches[i+1])
				if err != nil {
					return errors.Wrap(err, "error parsing IP: "+matches[i+1])
				}
				config.networkIP[i+12] = byte(ipByte)
			}
			testing.ContextLogf(ctx, "net=%v", config.networkIP)
		} else if matches := matcherNetmask.FindStringSubmatch(line); matches != nil {
			config.netmask = net.IPv4Mask(0, 0, 0, 0)
			for i := 0; i < 4; i++ {
				ipByte, err := strconv.Atoi(matches[i+1])
				if err != nil {
					return errors.Wrap(err, "error parsing IP: "+matches[i+1])
				}
				config.netmask[i] = byte(ipByte)
			}
			testing.ContextLogf(ctx, "mask=%v", config.netmask)
		}
	}

	return nil
}

func (ctrl *Controller) enableWLAN(ctx context.Context, id int) error {
	err := ctrl.sendCommandNoResult(ctx, fmt.Sprintf("config wlan enable %d", id))
	if err != nil {
		return errors.Wrap(err, "failed to enable wlan")
	}

	return nil
}

func (ctrl *Controller) findUnusedAP(ctx context.Context) (ap *apData) {
	for _, ap := range ctrl.aps {
		if ap.ap == nil {
			return ap
		}
	}

	return nil
}

func (ctrl *Controller) setupAPGroup(ctx context.Context, groupName string, wlan *wlanConfig) error {
	err := ctrl.sendCommandNoResult(ctx, "config wlan apgroup add "+groupName)
	if err != nil {
		return errors.Wrap(err, "failed to create AP group "+groupName)
	}

	err = ctrl.sendCommandNoResult(ctx, fmt.Sprintf(
		"config wlan apgroup interface-mapping add %s %d management", groupName, wlan.id))
	if err != nil {
		return errors.Wrap(err, "failed to assign WLAN to AP group "+groupName)
	}

	return nil
}

func (ctrl *Controller) assignAPToGroup(ctx context.Context, ap *apData, groupName string) error {
	cmd := fmt.Sprintf("config ap group-name %s %s", groupName, ap.name)
	testing.ContextLog(ctx, "assign="+cmd)
	_, err := ctrl.sendCommand(ctx, cmd, true)
	if err != nil {
		return errors.Wrap(err, "failed to assign AP to group "+groupName)
	}

	return nil
}

func (ctrl *Controller) deleteAllAPGroups(ctx context.Context) error {
	out, err := ctrl.sendCommand(ctx, "show wlan apgroups", true)
	if err != nil {
		return errors.Wrap(err, "failed to get AP groups")
	}

	testing.ContextLog(ctx, "groups="+out)

	lines := strings.Split(out, "\n")

	for _, line := range lines {
		if matches := matcherAPGroupName.FindStringSubmatch(line); matches != nil {
			groupName := matches[1]

			if groupName == defaultGroupName {
				continue
			}

			err := ctrl.sendCommandNoResult(ctx, "config wlan apgroup delete "+groupName)
			if err != nil {
				return errors.Wrap(err, "failed to delete AP group: "+groupName)
			}
		}
	}

	testing.ContextLog(ctx, "deleteAllAPGroups end")

	return nil
}

func (ctrl *Controller) clearAPConfig(ctx context.Context) error {
	out, err := ctrl.sendCommand(ctx, "show ap summary", true)
	if err != nil {
		return errors.Wrap(err, "failed to get APs summary")
	}

	testing.ContextLog(ctx, "aps="+out)

	lines := strings.Split(out, "\n")

	for _, line := range lines {
		if matches := matcherAPRow.FindStringSubmatch(line); matches != nil {
			apName := matches[1]

			out, err := ctrl.sendCommand(ctx, "clear ap config "+apName, true)
			if err != nil {
				return errors.Wrap(err, "failed to clear AP config: "+apName)
			}

			testing.ContextLog(ctx, "clear ap out="+out)
		}
	}

	return nil
}

func (ctrl *Controller) initAPs(ctx context.Context) error {
	out, err := ctrl.sendCommand(ctx, "show ap summary", true)
	if err != nil {
		return errors.Wrap(err, "failed to get APs")
	}

	ctrl.aps = make(map[string](*apData))

	testing.ContextLog(ctx, "aps="+out)

	lines := strings.Split(out, "\n")

	var apNames []string

	for _, line := range lines {
		if matches := matcherAPRow.FindStringSubmatch(line); matches != nil {
			var ap apData
			ap.name = matches[1]
			ap.ap = nil
			ctrl.aps[ap.name] = &ap
			apNames = append(apNames, ap.name)

			_, err = ctrl.sendCommand(ctx, fmt.Sprintf("config 802.11a disable %s", ap.name), true)
			if err != nil {
				return errors.Wrap(err, " failed to disable AP")
			}
			_, err = ctrl.sendCommand(ctx, fmt.Sprintf("config 802.11-abgn disable %s", ap.name), true)
			if err != nil {
				return errors.Wrap(err, " failed to disable AP")
			}
		}
	}

	testing.ContextLog(ctx, "Cisco APs available: "+strings.Join(apNames, ", "))

	return nil
}

func (ctrl *Controller) waitAPsOnline(ctx context.Context) error {
	numberAPs := 0

	for numberAPs != len(ctrl.aps) {
		out, err := ctrl.sendCommand(ctx, "show ap summary", true)
		if err != nil {
			return errors.Wrap(err, "failed to get APs")
		}

		matches := matcherNumberAPs.FindStringSubmatch(out)
		if matches == nil {
			return errors.New("Missing number of APs")
		}

		numberAPs, err = strconv.Atoi(matches[1])
		if err != nil {
			return errors.Wrap(err, "failed to parse number of APs")
		}
	}

	return nil
}

func (ctrl *Controller) waitAPOnline(ctx context.Context, ap *apData) error {
	pollFunc := func(ctx context.Context) error {
		out, err := ctrl.sendCommand(ctx, "show ap summary", true)
		if err != nil {
			return errors.Wrap(err, "failed to get APs")
		}

		lines := strings.Split(out, "\n")
		for _, line := range lines {
			if matches := matcherAPRow.FindStringSubmatch(line); matches != nil {
				if matches[1] == ap.name {
					return nil
				}
			}
		}

		return errors.New("no aps yet")
	}

	return testing.Poll(ctx, pollFunc, &apOnlinePollOptions)
}
