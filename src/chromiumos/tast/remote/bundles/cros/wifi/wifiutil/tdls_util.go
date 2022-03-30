// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/caller"
	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const verbose = false

// UniqueAPName returns AP name to be used in packet dumps.
func UniqueAPName() string {
	id := strconv.Itoa(apID)
	apID++
	return id
}

// ExpectOutput checks if string contains matching regexp.
func ExpectOutput(str, lookup string) bool {
	re := regexp.MustCompile(lookup)
	return re.MatchString(str)
}

// RunAndCheckOutput runs command and checks if the output matches expected regexp.
func RunAndCheckOutput(ctx context.Context, cmd ssh.Cmd, lookup string) (bool, error) {
	if verbose {
		testing.ContextLogf(ctx, "# %s", strings.Join(cmd.Args, " "))
	}
	ret, err := cmd.CombinedOutput()
	if verbose {
		testing.ContextLogf(ctx, "Output: %s", string(ret))
	}
	if err != nil {
		return false, errors.Wrap(err, "failed to call command, err")
	}
	return ExpectOutput(string(ret), lookup), nil
}

// WaitUntilExpcted retries command until the output matches expected regexp or timeout.
func WaitUntilExpcted(ctx context.Context, cmd ssh.Cmd, lookup string) (string, error) {
	var output string
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if verbose {
			testing.ContextLogf(ctx, "# %s", strings.Join(cmd.Args, " "))
		}
		cmdCpy := cmd
		ret, err := cmdCpy.Output()
		output = string(ret)
		if verbose {
			testing.ContextLogf(ctx, "Output: %s", string(ret))
		}
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to call command, err"))
		}
		if !ExpectOutput(string(ret), lookup) {
			testing.ContextLog(ctx, "Unexpected output. Waiting")
			testing.Sleep(ctx, 5*time.Second)
			return errors.Errorf("Command result: %s", ret)
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second})
	if err != nil {
		return "", errors.Wrap(err, "failed to reach desired output")
	}
	return output, err
}

// WpaCli runs (with retries) a wpa_cli command on provided connection and waits
// for the desired output.
func WpaCli(ctx context.Context, conn *ssh.Conn, result string, cmd ...string) (string, error) {
	runnerCtx, cancel := context.WithTimeout(ctx, 130*time.Second)
	defer cancel()
	args := []string{"-u", "wpa", "wpa_cli"}
	args = append(args, cmd...)

	output, err := WaitUntilExpcted(runnerCtx, *conn.CommandContext(runnerCtx, "sudo", args...), result)
	if err != nil {
		return "", errors.Wrap(err, "failed to run WPA CLI, err")
	}
	return output, nil
}

// GetMAC returns MAC address of the requested interface on the device accessible through SSH connection.
func GetMAC(ctx context.Context, conn *ssh.Conn, ifName string) (string, error) {
	ipr := ip.NewRemoteRunner(conn)
	hwMAC, err := ipr.MAC(ctx, ifName)
	if err != nil {
		return "", errors.Wrap(err, "failed to get MAC of WiFi interface")
	}
	return hwMAC.String(), nil
}

func collectFirstErr(ctx context.Context, firstErr *error, err error) {
	if err == nil {
		return
	}
	testing.ContextLogf(ctx, "Error in %s: %s", caller.Get(2), err)
	if *firstErr == nil {
		*firstErr = err
	}
}

// TDLSPeer defines chromeos-based wifi-capable host for TDLS tests purposes.
type TDLSPeer struct {
	host       *dut.DUT
	rpc        *rpc.Client
	wifiClient *wificell.WifiClient
}

// NewTDLSPeer connects to a new TDLSPeer device and returns its handle.
func NewTDLSPeer(ctx context.Context, peer, keyFile, keyDir string, rpcHint *testing.RPCHint) (*TDLSPeer, error) {
	var peerOpts ssh.Options
	tp := &TDLSPeer{}
	if err := ssh.ParseTarget(peer, &peerOpts); err != nil {
		return nil, errors.Wrap(err, "failed to parse peer data, err")
	}
	peerOpts.KeyDir = keyDir
	peerOpts.KeyFile = keyFile
	var err error
	tp.host, err = dut.New(peer, keyFile, keyDir, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create peer device object, err")
	}
	err = tp.host.Connect(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect peer device, err")
	}
	tp.rpc, err = rpc.Dial(ctx, tp.host, rpcHint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect rpc")
	}
	tp.wifiClient = &wificell.WifiClient{
		ShillServiceClient: wifi.NewShillServiceClient(tp.rpc.Conn),
	}
	return tp, nil
}

// Close tears down TDLSPeer.
func (tp *TDLSPeer) Close(ctx context.Context) error {
	var firstErr error
	if _, err := tp.wifiClient.TearDown(ctx, &empty.Empty{}); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to tear down WiFi client"))
	}
	if err := tp.rpc.Close(ctx); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to close RPC connection"))
	}
	if err := tp.host.Disconnect(ctx); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to disconnect from peer"))
	}
	return firstErr
}

// ConnectWifi asks the peer to connect to the specified WiFi.
func (tp *TDLSPeer) ConnectWifi(ctx context.Context, ap *wificell.APIface, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	conf := ap.Config()
	opts := append([]dutcfg.ConnOption{dutcfg.ConnHidden(conf.Hidden), dutcfg.ConnSecurity(conf.SecurityConfig)}, options...)
	c := &dutcfg.ConnConfig{
		Ssid:    conf.SSID,
		SecConf: &base.Config{},
	}
	for _, op := range opts {
		op(c)
	}

	secProps, err := c.SecConf.ShillServiceProperties()
	if err != nil {
		return nil, err
	}

	props := make(map[string]interface{})
	for k, v := range c.Props {
		props[k] = v
	}
	for k, v := range secProps {
		props[k] = v
	}

	propsEnc, err := protoutil.EncodeToShillValMap(props)
	if err != nil {
		return nil, err
	}
	request := &wifi.ConnectRequest{
		Ssid:       []byte(c.Ssid),
		Hidden:     c.Hidden,
		Security:   c.SecConf.Class(),
		Shillprops: propsEnc,
	}
	response, err := tp.wifiClient.Connect(ctx, request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// DisconnectWifi disconnects peer's WiFi.
func (tp *TDLSPeer) DisconnectWifi(ctx context.Context) error {
	resp, err := tp.wifiClient.SelectedService(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "failed to get selected service")
	}

	req := &wifi.DisconnectRequest{
		ServicePath:   resp.ServicePath,
		RemoveProfile: true,
	}
	if _, err := tp.wifiClient.Disconnect(ctx, req); err != nil {
		return errors.Wrap(err, "failed to disconnect")
	}
	return nil
}

// Conn returns ssh connection pointer to TDLSPeer device.
func (tp *TDLSPeer) Conn() *ssh.Conn {
	return tp.host.Conn()
}
