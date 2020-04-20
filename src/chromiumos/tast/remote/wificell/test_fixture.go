// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// TFOption is the function signature used to specify options of Config.
type TFOption func(*TestFixture)

// TFRouter sets the router hostname for the test fixture.
func TFRouter(target string) TFOption {
	return func(tf *TestFixture) {
		tf.routerTarget = target
	}
}

// TFPcap sets the pcap hostname for the test fixture.
func TFPcap(target string) TFOption {
	return func(tf *TestFixture) {
		tf.pcapTarget = target
	}
}

// TFCapture sets if the test fixture should spawn packet capturer.
func TFCapture(b bool) TFOption {
	return func(tf *TestFixture) {
		tf.packetCapture = b
	}
}

// TestFixture sets up the context for a basic WiFi test.
type TestFixture struct {
	dut        *dut.DUT
	rpc        *rpc.Client
	wifiClient network.WifiClient

	routerTarget  string
	routerHost    *ssh.Conn
	router        *Router
	pcapTarget    string
	pcapHost      *ssh.Conn
	pcap          *Router
	packetCapture bool

	apID       int
	curService *network.Service
	curAP      *APIface
	capturers  map[*APIface]*pcap.Capturer
}

// connectCompanion dials SSH connection to companion device with the auth key of DUT.
func (tf *TestFixture) connectCompanion(ctx context.Context, hostname string) (*ssh.Conn, error) {
	var sopt ssh.Options
	ssh.ParseTarget(hostname, &sopt)
	sopt.KeyDir = tf.dut.KeyDir()
	sopt.KeyFile = tf.dut.KeyFile()
	sopt.ConnectTimeout = 10 * time.Second
	return ssh.New(ctx, &sopt)
}

// NewTestFixture creates a TestFixture.
// The TestFixture contains a gRPC connection to the DUT and a SSH connection to the router.
// Noted that if routerHostname is empty, it uses the default router hostname based on the DUT's hostname.
func NewTestFixture(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, ops ...TFOption) (ret *TestFixture, retErr error) {
	tf := &TestFixture{
		dut:       dut,
		capturers: make(map[*APIface]*pcap.Capturer),
	}
	for _, op := range ops {
		op(tf)
	}

	defer func() {
		if retErr != nil {
			tf.Close(ctx)
		}
	}()
	var err error
	tf.rpc, err = rpc.Dial(ctx, tf.dut, rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect rpc")
	}
	tf.wifiClient = network.NewWifiClient(tf.rpc.Conn)

	// TODO(crbug.com/1034875): For now, we assume that we start with a clean DUT.
	// We may need a gRPC for initializing clean state on DUT. e.g. init_test_network_state
	// or WiFiClient.__init__ in Autotest.
	// TODO(crbug.com/728769): Make sure if we need to turn off powersave.

	if tf.routerTarget == "" {
		tf.routerHost, err = tf.dut.DefaultWifiRouterHost(ctx)
	} else {
		tf.routerHost, err = tf.connectCompanion(ctx, tf.routerTarget)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the router")
	}
	tf.router, err = NewRouter(ctx, tf.routerHost, "router")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a router object")
	}

	// TODO(crbug.com/1034875): Handle the case that routerTarget and pcapTarget
	// is pointing to the same device. Current Router object does not allow this.
	if tf.pcapTarget == "" {
		tf.pcapHost, err = tf.dut.DefaultWifiPcapHost(ctx)
	} else if tf.pcapTarget == tf.routerTarget {
		// Assign error here to use the same objects as router.
		err = errors.New("same target for router and pcap")
	} else {
		tf.pcapHost, err = tf.connectCompanion(ctx, tf.pcapTarget)
	}
	if err != nil {
		// Pcap server not reachable, use router instead.
		tf.pcapHost = tf.routerHost
		tf.pcap = tf.router
	} else {
		tf.pcap, err = NewRouter(ctx, tf.pcapHost, "pcap")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create router object for pcap")
		}
	}

	// TODO(crbug.com/1034875): set up attenuator.

	// Seed the random as we have some randomization. e.g. default SSID.
	rand.Seed(time.Now().UnixNano())
	return tf, nil
}

// Close closes the connections created by TestFixture.
func (tf *TestFixture) Close(ctx context.Context) error {
	var retErr error
	if tf.pcap != nil && tf.pcap != tf.router {
		if err := tf.pcap.Close(ctx); err != nil {
			retErr = errors.Wrapf(err, "failed to close pcap: %s", err.Error())
		}
	}
	if tf.pcapHost != nil && tf.pcapHost != tf.routerHost {
		if err := tf.pcapHost.Close(ctx); err != nil {
			retErr = errors.Wrapf(err, "failed to close pcap ssh: %s", err.Error())
		}
	}
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

// getUniqueAPName returns an unique ID string for each AP as their name, so that related
// logs/pcap can be identified easily.
func (tf *TestFixture) getUniqueAPName() string {
	id := strconv.Itoa(tf.apID)
	tf.apID++
	return id
}

// ConfigureAP configures the router to provide a WiFi service with the options specified.
func (tf *TestFixture) ConfigureAP(ctx context.Context, ops ...hostapd.Option) (ret *APIface, retErr error) {
	name := tf.getUniqueAPName()
	config, err := hostapd.NewConfig(ops...)
	if err != nil {
		return nil, err
	}
	ap, err := tf.router.StartAPIface(ctx, name, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start APIface")
	}

	if tf.packetCapture {
		ops, err := config.PcapFreqOptions()
		if err != nil {
			tf.router.StopAPIface(ctx, ap)
			return nil, err
		}
		capturer, err := tf.pcap.StartCapture(ctx, name, config.Channel, ops)
		if err != nil {
			tf.router.StopAPIface(ctx, ap)
			return nil, errors.Wrap(err, "failed to start capturer")
		}
		tf.capturers[ap] = capturer
	}
	return ap, nil
}

// DeconfigAP stops the WiFi service on router.
func (tf *TestFixture) DeconfigAP(ctx context.Context, ap *APIface) error {
	var retErr error
	capturer := tf.capturers[ap]
	delete(tf.capturers, ap)
	if capturer != nil {
		if err := tf.pcap.StopCapture(ctx, capturer); err != nil {
			retErr = errors.Wrapf(retErr, "failed to stop capturer, err=%s", err.Error())
		}
	}
	if err := tf.router.StopAPIface(ctx, ap); err != nil {
		retErr = errors.Wrapf(retErr, "failed to stop APIface, err=%s", err.Error())
	}
	return retErr
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

// Pcap returns the pcap Router object in the fixture.
func (tf *TestFixture) Pcap() *Router {
	return tf.pcap
}

// WifiClient returns the gRPC WifiClient of the DUT.
func (tf *TestFixture) WifiClient() network.WifiClient {
	return tf.wifiClient
}
