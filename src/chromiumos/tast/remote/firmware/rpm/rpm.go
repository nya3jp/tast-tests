// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rpm

import (
	"context"
	"time"

	"chromiumos/tast/common/xmlrpc"
)

// PowerState is a state to be passed to SetPowerViaPoe or SetPowerViaRPM.
type PowerState string

// States to be passed to SetPowerViaPoe or SetPowerViaRPM.
const (
	Off   PowerState = "OFF"
	On    PowerState = "ON"
	Cycle PowerState = "CYCLE"
)

// RPM is the client interface to the remote power management service.
type RPM struct {
	xmlrpc *xmlrpc.XMLRpc
}

// Use the RemoteRPMHost if you are outside of the lab, and LocalRPMHost if inside.
const (
	RemoteRPMHost  string = "chromeos-rpm-server.mtv.corp.google.com"
	LocalRPMHost   string = "rpm-service"
	DefaultRPMPort int    = 9999
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

// SetPowerViaPoe sets the power state for a plug attached to a dut that is using servo v3.
// There do not appear to be any of these left in the lab.
func (r *RPM) SetPowerViaPoe(ctx context.Context, hostname string, state PowerState) (bool, error) {
	var val bool
	err := r.xmlrpc.Run(ctx, xmlrpc.NewCallTimeout("set_power_via_poe", 2*time.Minute, hostname, string(state)), &val)
	return val, err
}

// SetPowerViaRPM sets the power state for a plug managed by RPM. `hydraHostname` is optional, the other params are required.
func (r *RPM) SetPowerViaRPM(ctx context.Context, hostname, powerunitHostname, powerunitOutlet, hydraHostname string, state PowerState) (bool, error) {
	var val bool
	err := r.xmlrpc.Run(ctx, xmlrpc.NewCallTimeout("set_power_via_rpm", 2*time.Minute, hostname, powerunitHostname, powerunitOutlet, hydraHostname, string(state)), &val)
	return val, err
}
