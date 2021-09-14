// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rpm

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/xmlrpc"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
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

	// restoreRPMPower is set if we need to enable power on the plug on close.
	restoreRPMPower bool

	// dutHostname is the real name of the dut, even if tast is connected to a forwarded port.
	dutHostname string

	// powerunitHostname, powerunitOutlet, hydraHostname identify the managed power outlet for the DUT.
	powerunitHostname, powerunitOutlet, hydraHostname string
}

// Use the RemoteRPMHost if you are outside of the lab, and LocalRPMHost if inside.
const (
	RemoteRPMHost  string = "chromeos-rpm-server.mtv.corp.google.com"
	LocalRPMHost   string = "rpm-service"
	DefaultRPMPort int    = 9999
)

// NewLabRPM creates a new RPM object for communicating with a RPM server in the lab.
// `hydraHostname` is optional, the other params are required.
func NewLabRPM(ctx context.Context, pxy *servo.Proxy, dutHostname, powerunitHostname, powerunitOutlet, hydraHostname string) (*RPM, error) {
	rpmHost := LocalRPMHost
	port := DefaultRPMPort
	if _, err := net.ResolveIPAddr("ip", rpmHost); err != nil {
		testing.ContextLogf(ctx, "Could not resolve %q: %s", rpmHost, err)
		rpmHost = RemoteRPMHost
		// When running outside of the lab, the rpm server is not accessible, so proxy through the servo host.
		if pxy != nil {
			fwd, err := pxy.NewForwarder(ctx, fmt.Sprintf("%s:%d", rpmHost, DefaultRPMPort))
			if err != nil {
				return nil, errors.Wrap(err, "forwarding rpm port")
			}
			var portstr string
			rpmHost, portstr, err = net.SplitHostPort(fwd.ListenAddr().String())
			if err != nil {
				return nil, errors.Wrap(err, "splitting host port")
			}
			port, err = strconv.Atoi(portstr)
			if err != nil {
				return nil, errors.Wrap(err, "parsing forwarded servo port")
			}
		}
	}
	r := &RPM{
		xmlrpc:            xmlrpc.New(rpmHost, port),
		dutHostname:       dutHostname,
		powerunitHostname: powerunitHostname,
		powerunitOutlet:   powerunitOutlet,
		hydraHostname:     hydraHostname,
	}

	return r, nil
}

// Close performs RPM cleanup.
func (r *RPM) Close(ctx context.Context) error {
	if r.restoreRPMPower {
		testing.ContextLog(ctx, "Restoring RPM power")
		if ok, err := r.SetPower(ctx, On); err != nil {
			return errors.Wrap(err, "failed to restore rpm power")
		} else if !ok {
			return errors.New("failed to restore rpm power")
		}
	}
	return nil
}

// SetPower sets the power state for a plug managed by RPM.
// Returns the bool returned by the xml rpc call, or error if the call failed.
// It is unclear under which situations the api will return false with no error.
func (r *RPM) SetPower(ctx context.Context, state PowerState) (bool, error) {
	var success bool
	err := r.xmlrpc.Run(ctx, xmlrpc.NewCallTimeout("set_power_via_rpm", 2*time.Minute, r.dutHostname, r.powerunitHostname, r.powerunitOutlet, r.hydraHostname, string(state)), &success)
	if err != nil {
		return false, errors.Wrap(err, "set power via rpm")
	}
	if success {
		r.restoreRPMPower = state == Off
	}
	return success, err
}
