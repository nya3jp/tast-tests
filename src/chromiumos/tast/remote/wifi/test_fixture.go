// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"math/rand"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wifi/hostap"
	"chromiumos/tast/remote/wifi/hostap/secconf"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

// TestFixture sets up the context for a basic WiFi test.
type TestFixture struct {
	dut        *dut.DUT
	rpc        *rpc.Client
	routerHost *host.SSH
	router     *Router
	curService *network.Service
	curHostAP  *HostAPHandle
}

const (
	// VarRouter is the variable used for TestContext to get router hostname.
	VarRouter = "router"
)

// NewTestFixture connects to rotuer and gRPC on DUT and then returns a TestFixture.
func NewTestFixture(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, routerHostname string) (ret *TestFixture, retErr error) {
	tc := &TestFixture{}
	defer func() {
		if retErr != nil {
			tc.Close(ctx)
		}
	}()
	var err error
	tc.dut = dut
	tc.rpc, err = rpc.Dial(ctx, dut, rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect rpc")
	}
	if routerHostname == "" {
		tc.routerHost, err = dut.DefaultWifiRouterHost(ctx)
	} else {
		var sopt host.SSHOptions
		sopt.Hostname = routerHostname
		sopt.KeyDir = dut.KeyDir()
		sopt.KeyFile = dut.KeyFile()
		sopt.ConnectTimeout = 10 * time.Second
		tc.routerHost, err = host.NewSSH(ctx, &sopt)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect router")
	}
	tc.router, err = NewRouter(ctx, tc.routerHost)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create router object")
	}
	// Seed the random as we have some randomization. e.g. default SSID.
	rand.Seed(time.Now().UnixNano())
	return tc, nil
}

// Close the connections created by TestFixture.
func (tc *TestFixture) Close(ctx context.Context) error {
	var err error
	if tc.router != nil {
		if err2 := tc.router.Close(ctx); err2 != nil {
			err = errors.Wrap(err2, "failed to close rotuer")
		}
	}
	if tc.routerHost != nil {
		if err2 := tc.routerHost.Close(ctx); err2 != nil {
			err = errors.Wrap(err2, "failed to close router ssh")
		}
	}
	if tc.rpc != nil {
		// Ignore the error of rpc.Close as aborting rpc daemon will always have error.
		tc.rpc.Close(ctx)
	}
	// Do not close DUT, it'll be closed by the framework.
	return err
}

// ConfigureAP configures the router to provide a wifi service with the options specified.
func (tc *TestFixture) ConfigureAP(ctx context.Context, ops ...hostap.Option) (*HostAPHandle, error) {
	apConf := hostap.NewConfig(ops...)
	return tc.router.StartHostAP(ctx, apConf)
}

// DeconfigAP stops the wifi service on router.
func (tc *TestFixture) DeconfigAP(ctx context.Context, ap *HostAPHandle) error {
	return tc.router.StopHostAP(ctx, ap)
}

// ConnectWifi asks DUT to connect to the given wifi service.
func (tc *TestFixture) ConnectWifi(ctx context.Context, ap *HostAPHandle) error {
	wc := network.NewWifiClient(tc.rpc.Conn)

	shillprops, err := secconf.EncodeToShillValMap(ap.Config().SecurityConfig.GetShillServiceProperties())
	if err != nil {
		return err
	}

	config := &network.Config{
		Ssid:       ap.Config().Ssid,
		Hidden:     ap.Config().Hidden,
		Security:   ap.Config().SecurityConfig.GetClass(),
		Shillprops: shillprops,
	}
	service, err := wc.Connect(ctx, config)
	if err != nil {
		return err
	}
	tc.curService = service
	tc.curHostAP = ap
	return nil
}

// DisconnectWifi asks DUT to disconnect from current wifi service and removes the configuration.
func (tc *TestFixture) DisconnectWifi(ctx context.Context) error {
	var err error
	wc := network.NewWifiClient(tc.rpc.Conn)
	if _, err2 := wc.Disconnect(ctx, tc.curService); err2 != nil {
		err = errors.Wrap(err2, "failed to disconnect")
	}
	ssid := tc.curHostAP.Config().Ssid
	if _, err2 := wc.DeleteEntriesForSSID(ctx, &network.SSID{Ssid: ssid}); err2 != nil {
		err = errors.Wrapf(err, "failed to delete entries for ssid=%q, err=%q", ssid, err2.Error())
	}
	tc.curService = nil
	tc.curHostAP = nil
	return err
}

// PingFromDUT tests the connectivity between DUT and router through currently connected wifi service.
func (tc *TestFixture) PingFromDUT(ctx context.Context) error {
	if tc.curHostAP == nil {
		return errors.New("not connected")
	}
	pr := ping.NewRunner(tc.dut)
	res, err := pr.Ping(ctx, tc.curHostAP.ServerIP().String())
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "ping statistics=%+v", res)
	if res.Sent != res.Received {
		return errors.New("Some packets are lost in ping")
	}
	return nil
}
