// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

// This file contains the utilities shared by Precondition and Fixture.
// We might merge these back to fixture.go and improve the code structure,
// once we deprecate Precondition.

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

// dutHealthCheck checks if the DUT is healthy.
func dutHealthCheck(ctx context.Context, d *dut.DUT, rpcHint *testing.RPCHint) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// We create a new gRPC session here to exclude broken gRPC case and save reboots when
	// the DUT is healthy but the gRPC is broken.
	rpcClient, err := rpc.Dial(ctx, d, rpcHint, "cros")
	if err != nil {
		return errors.Wrap(err, "cannot create gRPC client")
	}
	defer rpcClient.Close(ctx)

	wifiClient := wifi.NewShillServiceClient(rpcClient.Conn)
	if _, err := wifiClient.HealthCheck(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "health check failed")
	}
	return nil
}

// recoverUnhealthyDUT checks if the DUT is healthy. If not, close existing
// TestFxiture and try to recover it with reboot.
func recoverUnhealthyDUT(ctx context.Context, d *dut.DUT, rpcHint *testing.RPCHint, existingTF **TestFixture) error {
	if err := dutHealthCheck(ctx, d, rpcHint); err != nil {
		testing.ContextLog(ctx, "Rebooting the DUT due to health check err: ", err)
		// As reboot will at least break tf.rpc, no reason to keep
		// the existing p.tf. Close it before reboot.
		if existingTF != nil && *existingTF != nil {
			testing.ContextLog(ctx, "Close TestFixture before reboot")
			if err := (*existingTF).Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close TestFixture before DUT reboot recovery: ", err)
			}
			*existingTF = nil
		}
		if err := d.Reboot(ctx); err != nil {
			return errors.Wrap(err, "reboot failed")
		}
	}
	return nil
}

func companionName(d *dut.DUT, suffix string) (string, error) {
	return d.CompanionDeviceHostname(suffix)
}

func setUpTestFixture(ctx, daemonCtx context.Context, d *dut.DUT, rpcHint *testing.RPCHint, features TFFeatures, getVar func(string) (string, bool)) (*TestFixture, error) {
	// Create TestFixture.
	var ops []TFOption
	// Read router/pcap variable. If not available or empty, NewTestFixture
	// will fall back to Default{Router,Pcap}Host.
	if features&TFFeaturesRouters != 0 {
		if routers, ok := getVar("routers"); ok && routers != "" {
			testing.ContextLog(ctx, "routers: ", routers)
			slice := strings.Split(routers, ",")
			if len(slice) < 2 {
				return nil, errors.New("must provide at least two router names when Routers feature is enabled")
			}
			ops = append(ops, TFRouter(slice...))
		} else {
			var routers []string
			for _, suffix := range []string{dut.CompanionSuffixRouter, dut.CompanionSuffixPcap} {
				name, err := companionName(d, suffix)
				if err != nil {
					return nil, err
				}
				routers = append(routers, name)

			}
			testing.ContextLog(ctx, "companion routers: ", routers)
			ops = append(ops, TFRouter(routers...))
		}
	} else {
		router, ok := getVar("router")
		if ok && router != "" {
			testing.ContextLog(ctx, "router: ", router)
			ops = append(ops, TFRouter(router))
		} // else: let TestFixture resolve the name.
	}
	pcap, ok := getVar("pcap")
	if ok && pcap != "" {
		testing.ContextLog(ctx, "pcap: ", pcap)
		ops = append(ops, TFPcap(pcap))
	} // else: let TestFixture resolve the name.
	if features&TFFeaturesRouterAsCapture != 0 {
		testing.ContextLog(ctx, "using router as pcap")
		ops = append(ops, TFRouterAsCapture())
	}
	// Read attenuator variable.
	if features&TFFeaturesAttenuator != 0 {
		atten, ok := getVar("attenuator")
		if !ok || atten == "" {
			// Attenuator is not typical companion, so we synthesize its name here.
			var err error
			if atten, err = companionName(d, "-attenuator"); err != nil {
				return nil, err
			}
		}
		testing.ContextLog(ctx, "attenuator: ", atten)
		ops = append(ops, TFAttenuator(atten))
	}
	// Enable capturing.
	if features&TFFeaturesCapture != 0 {
		ops = append(ops, TFCapture(true))
	}
	return NewTestFixture(ctx, daemonCtx, d, rpcHint, ops...)
}
