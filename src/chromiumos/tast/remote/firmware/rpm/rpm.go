// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rpm

import (
	"context"
	"time"

	"chromiumos/tast/common/xmlrpc"
)

type PowerState string

const (
	Off   PowerState = "OFF"
	On    PowerState = "ON"
	Cycle PowerState = "CYCLE"
)

type RPM struct {
	xmlrpc *xmlrpc.XMLRpc
}

const (
	DefaultRPMServer string = "chromeos-rpm-server.mtv.corp.google.com"
	DefaultRPMPort   int    = 9999
)

// New creates a new RPM object for communicating with a RPM server.
func New(ctx context.Context, host string, port int) (*RPM, error) {
	r := &RPM{
		xmlrpc: xmlrpc.New(host, port),
	}

	return r, nil
}

// Close performs RPM cleanup.
func (r *RPM) Close(ctx context.Context) error {
	return nil
}

func (r *RPM) SetPowerViaPoe(ctx context.Context, hostname string, state PowerState) (bool, error) {
	var val bool
	err := r.xmlrpc.Run(ctx, xmlrpc.NewCallTimeout("set_power_via_poe", 2*time.Minute, hostname, string(state)), &val)
	return val, err
}

func (r *RPM) SetPowerViaRPM(ctx context.Context, hostname, powerunitHostname, powerunitOutlet, hydraHostname string, state PowerState) (bool, error) {
	var val bool
	err := r.xmlrpc.Run(ctx, xmlrpc.NewCallTimeout("set_power_via_rpm", 2*time.Minute, hostname, powerunitHostname, powerunitOutlet, hydraHostname, string(state)), &val)
	return val, err
}
