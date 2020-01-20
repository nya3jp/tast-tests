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
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wifi/hostap"
	"chromiumos/tast/remote/wifi/utils"
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
	// TODO(yenlinlai): We may need a gRPC for initializing clean state on DUT.
	// e.g. init_test_network_state in Autotest.
	if routerHostname == "" {
		tf.routerHost, err = dut.DefaultWifiRouterHost(ctx)
	} else {
		var sopt host.SSHOptions
		sopt.Hostname = routerHostname
		sopt.KeyDir = dut.KeyDir()
		sopt.KeyFile = dut.KeyFile()
		sopt.ConnectTimeout = 10 * time.Second
		tf.routerHost, err = host.NewSSH(ctx, &sopt)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect router")
	}
	tf.router, err = NewRouter(ctx, tf.routerHost)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create router object")
	}
	if err := utils.SyncTime(ctx, tf.dut); err != nil {
		return nil, errors.Wrap(err, "failed to sync DUT time")
	}
	if err := utils.SyncTime(ctx, tf.routerHost); err != nil {
		return nil, errors.Wrap(err, "failed to sync router time")
	}
	// Seed the random as we have some randomization. e.g. default SSID.
	rand.Seed(time.Now().UnixNano())
	return tf, nil
}

// Close the connections created by TestFixture.
func (tf *TestFixture) Close(ctx context.Context) error {
	var err error
	if tf.router != nil {
		if err2 := tf.router.Close(ctx); err2 != nil {
			err = errors.Wrap(err2, "failed to close rotuer")
		}
	}
	if tf.routerHost != nil {
		if err2 := tf.routerHost.Close(ctx); err2 != nil {
			err = errors.Wrap(err2, "failed to close router ssh")
		}
	}
	if tf.rpc != nil {
		// Ignore the error of rpc.Close as aborting rpc daemon will always have error.
		tf.rpc.Close(ctx)
	}
	// Do not close DUT, it'll be closed by the framework.
	return err
}

// ConfigureAP configures the router to provide a wifi service with the options specified.
func (tf *TestFixture) ConfigureAP(ctx context.Context, ops ...hostap.Option) (*HostAPHandle, error) {
	apConf := hostap.NewConfig(ops...)
	return tf.router.StartHostAP(ctx, apConf)
}

// DeconfigAP stops the wifi service on router.
func (tf *TestFixture) DeconfigAP(ctx context.Context, ap *HostAPHandle) error {
	return tf.router.StopHostAP(ctx, ap)
}

// ConnectWifi asks DUT to connect to the given wifi service.
func (tf *TestFixture) ConnectWifi(ctx context.Context, ap *HostAPHandle) error {
	wc := network.NewWifiClient(tf.rpc.Conn)

	config := &network.Config{
		Ssid: ap.Config().Ssid,
	}
	service, err := wc.Connect(ctx, config)
	if err != nil {
		return err
	}
	tf.curService = service
	tf.curHostAP = ap
	return nil
}

// DisconnectWifi asks DUT to disconnect from current wifi service and removes the configuration.
func (tf *TestFixture) DisconnectWifi(ctx context.Context) error {
	var err error
	wc := network.NewWifiClient(tf.rpc.Conn)
	if _, err2 := wc.Disconnect(ctx, tf.curService); err2 != nil {
		err = errors.Wrap(err2, "failed to disconnect")
	}
	ssid := tf.curHostAP.Config().Ssid
	if _, err2 := wc.DeleteEntriesForSSID(ctx, &network.SSID{Ssid: ssid}); err2 != nil {
		err = errors.Wrapf(err, "failed to delete entries for ssid=%q, err=%q", ssid, err2.Error())
	}
	tf.curService = nil
	tf.curHostAP = nil
	return err
}

// PingFromDUT tests the connectivity between DUT and router through currently connected wifi service.
func (tf *TestFixture) PingFromDUT(ctx context.Context) error {
	if tf.curHostAP == nil {
		return errors.New("not connected")
	}
	pr := ping.NewRunner(tf.dut)
	res, err := pr.Ping(ctx, tf.curHostAP.ServerIP().String())
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "ping statistics=%+v", res)
	if res.Sent != res.Received {
		return errors.New("Some packets are lost in ping")
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
	err = el.Stop(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to stop iw.EventLogger")
	}
	if errf != nil {
		return errf
	}
	evs := el.DisconnectEvents()
	if len(evs) != 0 {
		return errors.Errorf("disconnect events captured: %v", evs)
	}
	return nil
}
