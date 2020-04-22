// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// TestFixture sets up the context for a basic WiFi test.
type TestFixture struct {
	dut        *dut.DUT
	rpc        *rpc.Client
	routerHost *ssh.Conn
	router     *Router
	wifiClient network.WifiClient

	apID       int
	curService *network.Service
	curAP      *APIface
}

// NewTestFixture creates a TestFixture with a shortCtx.
// The TestFixture contains a gRPC connection to the DUT and a SSH connection to the router.
// Noted that if routerHostname is empty, it uses the default router hostname based on the DUT's hostname.
// TestFixture user shall call tf.Close() to perform clean-up. And shortCtx is used to reserve time for the
// clean-up function to run. For example:
//   tf, ctx, ctxCancel, err := NewTestFixture(fullCtx, ...)
//   if err != nil {
//     s.Fatal("Failed to create TestFixture: ", err)
//   }
//   defer tf.Close(fullCtx)
//   defer ctxCancel()
//   // The rest of the code run things with ctx...
func NewTestFixture(fullCtx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, routerTarget string) (
	ret *TestFixture, shortCtx context.Context, shortCtxCancel context.CancelFunc, retErr error) {
	fullCtx, st := timing.Start(fullCtx, "NewTestFixture")
	defer st.End()

	tf := &TestFixture{}
	// Shorten fullCtx for cleaning up tf.rpc and tf.routerHost.
	// The context will be shortened again when obtaining a Router.
	// We only need to call cancel of the first shortened context because the shorten context's Done
	// channel is closed when the parent context's Done channel is closed.
	sCtx, sCtxCancel := ctxutil.Shorten(fullCtx, 5*time.Second)

	defer func() {
		if retErr != nil {
			sCtxCancel()
			tf.Close(fullCtx)
		}
	}()

	var err error
	tf.dut = dut
	tf.rpc, err = rpc.Dial(sCtx, dut, rpcHint, "cros")
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to connect rpc")
	}
	tf.wifiClient = network.NewWifiClient(tf.rpc.Conn)

	// TODO(crbug.com/1034875): For now, we assume that we start with a clean DUT.
	// We may need a gRPC for initializing clean state on DUT. e.g. init_test_network_state
	// or WiFiClient.__init__ in Autotest.
	// TODO(crbug.com/728769): Make sure if we need to turn off powersave.

	if routerTarget == "" {
		tf.routerHost, err = dut.DefaultWifiRouterHost(sCtx)
	} else {
		var sopt ssh.Options
		ssh.ParseTarget(routerTarget, &sopt)
		sopt.KeyDir = dut.KeyDir()
		sopt.KeyFile = dut.KeyFile()
		sopt.ConnectTimeout = 10 * time.Second
		tf.routerHost, err = ssh.New(sCtx, &sopt)
	}
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to connect to the router")
	}
	r, sCtx1, _, err := NewRouter(sCtx, tf.routerHost, "router")
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to create a router object")
	}
	tf.router = r

	// TODO(crbug.com/1034875): set up pcap, attenuator.

	// Seed the random as we have some randomization. e.g. default SSID.
	rand.Seed(time.Now().UnixNano())
	return tf, sCtx1, sCtxCancel, nil
}

// Close closes the connections created by TestFixture.
func (tf *TestFixture) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "tf.Close")
	defer st.End()

	var retErr error
	if tf.router != nil {
		if err := tf.router.Close(ctx); err != nil {
			retErr = errors.Wrapf(retErr, "failed to close router: %s", err.Error())
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

// getUniqueAPName returns an unique ID string for each AP as their name, so that related
// logs/pcap can be identified easily.
func (tf *TestFixture) getUniqueAPName() string {
	id := strconv.Itoa(tf.apID)
	tf.apID++
	return id
}

// ConfigureAP configures the router and returns an APIface object with a shortened context.
// It configures the router to provide a WiFi service with the options specified.
// The caller should call tf.DeconfigAP() to perform clean-up. And the shortened context is used to
// reserve time for the clean-up function to run. For example:
//   tf, tfCtx, tfCtxCancel, err := NewTestFixture(fullCtx, ...)
//   ...
//   ap, apCtx, apCtxCancel, err := tf.ConfigureAP(tfCtx, ...)
//   if err != nil {
//     return err
//   }
//   defer ap.DecofnigAP(tfCtx)
//   defer apCtxCancel()
//   // The rest of the code run things with apCtxc...
func (tf *TestFixture) ConfigureAP(ctx context.Context, ops ...hostapd.Option) (
	*APIface, context.Context, context.CancelFunc, error) {
	ctx, st := timing.Start(ctx, "tf.ConfigureAP")
	defer st.End()

	name := tf.getUniqueAPName()
	config, err := hostapd.NewConfig(ops...)
	if err != nil {
		return nil, nil, nil, err
	}
	return tf.router.StartAPIface(ctx, name, config)
}

// DeconfigAP stops the WiFi service on router.
func (tf *TestFixture) DeconfigAP(ctx context.Context, h *APIface) error {
	ctx, st := timing.Start(ctx, "tf.DeconfigAP")
	defer st.End()

	return tf.router.StopAPIface(ctx, h)
}

// ConnectWifi asks the DUT to connect to the given WiFi service.
func (tf *TestFixture) ConnectWifi(ctx context.Context, h *APIface) error {
	ctx, st := timing.Start(ctx, "tf.ConnectWifi")
	defer st.End()

	config := &network.Config{
		Ssid:   h.Config().Ssid,
		Hidden: h.Config().Hidden,
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
	ctx, st := timing.Start(ctx, "tf.DisconnectWifi")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "tf.PingFromDUT")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "tf.AssertNoDisconnect")
	defer st.End()

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
