// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"math/rand"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell/hostapd"
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
	wifiClient network.WifiClient
	curService *network.Service
	curAP      *APIface
}

// NewTestFixture creates a TestFixture.
// The TestFixture contains a gRPC connection to the DUT and a SSH connection to the router.
// Noted that if routerHostname is empty, it uses the default router hostname based on the DUT's hostname.
func NewTestFixture(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, routerTarget string) (ret *TestFixture, retErr error) {
	tf := &TestFixture{}
	defer func() {
		if retErr != nil {
			tf.Close(ctx)
		}
	}()
	var err error
	tf.dut = dut
	tf.rpc, err = rpc.Dial(ctx, dut, rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect rpc")
	}
	tf.wifiClient = network.NewWifiClient(tf.rpc.Conn)

	// TODO(crbug.com/1034875): For now, we assume that we start with a clean DUT.
	// We may need a gRPC for initializing clean state on DUT. e.g. init_test_network_state
	// or WiFiClient.__init__ in Autotest.
	// TODO(crbug.com/728769): Make sure if we need to turn off powersave.

	if routerTarget == "" {
		tf.routerHost, err = dut.DefaultWifiRouterHost(ctx)
	} else {
		var sopt host.SSHOptions
		host.ParseSSHTarget(routerTarget, &sopt)
		sopt.KeyDir = dut.KeyDir()
		sopt.KeyFile = dut.KeyFile()
		sopt.ConnectTimeout = 10 * time.Second
		tf.routerHost, err = host.NewSSH(ctx, &sopt)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the router")
	}
	tf.router, err = NewRouter(ctx, tf.routerHost)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a router object")
	}

	// TODO(crbug.com/1034875): set up pcap, attenuator.

	// Seed the random as we have some randomization. e.g. default SSID.
	rand.Seed(time.Now().UnixNano())
	return tf, nil
}

// Close closes the connections created by TestFixture.
func (tf *TestFixture) Close(ctx context.Context) error {
	var retErr error
	if tf.router != nil {
		if err := tf.router.Close(ctx); err != nil {
			retErr = errors.Wrapf(retErr, "failed to close rotuer: %s", err.Error())
		}
	}
	if tf.routerHost != nil {
		if err := tf.routerHost.Close(ctx); err != nil {
			retErr = errors.Wrapf(retErr, "failed to close router ssh: %s", err.Error())
		}
	}
	if tf.rpc != nil {
		// Ignore the error of rpc.Close as aborting rpc daemon will always have error.
		tf.rpc.Close(ctx)
	}
	// Do not close DUT, it'll be closed by the framework.
	return retErr
}

// ConfigureAP configures the router to provide a WiFi service with the options specified.
func (tf *TestFixture) ConfigureAP(ctx context.Context, ops ...hostapd.Option) (*APIface, error) {
	config, err := hostapd.NewConfig(ops...)
	if err != nil {
		return nil, err
	}
	return tf.router.StartAPIface(ctx, config)
}

// DeconfigAP stops the WiFi service on router.
func (tf *TestFixture) DeconfigAP(ctx context.Context, h *APIface) error {
	return tf.router.StopAPIface(ctx, h)
}

// ConnectWifi asks the DUT to connect to the given WiFi service.
func (tf *TestFixture) ConnectWifi(ctx context.Context, h *APIface) error {
	config := &network.Config{
		Ssid: h.Config().Ssid,
	}
	service, err := tf.wifiClient.Connect(ctx, config)
	if err != nil {
		return err
	}
	tf.curService = service
	tf.curAP = h
	return nil
}

// DisconnectWifi asks the DUT to disconnect from current WiFi service and removes the configuration.
func (tf *TestFixture) DisconnectWifi(ctx context.Context) error {
	var err error
	if _, err2 := tf.wifiClient.Disconnect(ctx, tf.curService); err2 != nil {
		err = errors.Wrap(err2, "failed to disconnect")
	}
	tf.curService = nil
	tf.curAP = nil
	return err
}

// PingFromDUT tests the connectivity between DUT and router through currently connected WiFi service.
func (tf *TestFixture) PingFromDUT(ctx context.Context, opts ...ping.Option) error {
	if tf.curAP == nil {
		return errors.New("not connected")
	}
	pr := ping.NewRunner(tf.dut)
	res, err := pr.Ping(ctx, tf.curAP.ServerIP().String(), opts...)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "ping statistics=%+v", res)
	if res.Sent != res.Received {
		return errors.New("some packets are lost in ping")
	}
	return nil
}

// AssertNoDisconnect runs the given routine and verifies that no disconnection event
// is captured in the same duration.
func (tf *TestFixture) AssertNoDisconnect(ctx context.Context, f func(context.Context) error) error {
	el, err := iw.NewEventLogger(ctx, tf.dut)
	if err != nil {
		return errors.Wrap(err, "failed to start iw.EventLogger")
	}
	errf := f(ctx)
	if err := el.Stop(ctx); err != nil {
		// Let's also keep errf if available. Wrapf is equivalent to Errorf when errf==nil.
		return errors.Wrapf(errf, "failed to stop iw.EventLogger, err=%s", err.Error())
	}
	if errf != nil {
		return errf
	}
	evs := el.EventsByType(iw.EventTypeDisconnect)
	if len(evs) != 0 {
		return errors.Errorf("disconnect events captured: %v", evs)
	}
	return nil
}

// Router returns the Router object in the fixture.
func (tf *TestFixture) Router() *Router {
	return tf.router
}

// WifiClient returns the gRPC WifiClient of the DUT.
func (tf *TestFixture) WifiClient() network.WifiClient {
	return tf.wifiClient
}
