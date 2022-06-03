// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	resetshill "chromiumos/tast/local/bundles/cros/network/shill"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

var (
	okHeader = []byte("HTTP/1.0 200")
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCaptivePortalHTTP,
		Desc:     "Ensures that setting up a virtual ethernet pair with an http server results in a service state of Online",
		Contacts: []string{"michaelrygiel@google.com", "cros-networking@google.com"},
		Attr:     []string{},
	})
}

func ShillCaptivePortalHTTP(ctx context.Context, s *testing.State) {
	testing.ContextLog(ctx, "Restarting shill")
	if err := resetshill.ResetShill(ctx); err != nil {
		s.Fatal("Failed to reset shill: ", err)
	}
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}
	testing.ContextLog(ctx, "Disabling portal detection on ethernet")
	if err := m.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "wifi,cellular"); err != nil {
		s.Fatal("Failed to disable portal detection on ethernet: ", err)
	}

	testing.ContextLog(ctx, "Setting up an env for captive portal")
	options := virtualnet.CaptivePortalOptions{
		Priority:         5,
		AddressToForceIP: "www.gstatic.com",
	}
	portal, err := virtualnet.CreateCaptivePortalEnv(ctx, m, options)
	if err != nil {
		s.Fatal("Failed to create a portal env: ", err)
	}
	defer portal.Cleanup(ctx)

	s.Log("Connecting to http server")
	var addrs *env.IfaceAddrs
	if addrs, err = portal.GetVethInAddrs(ctx); err != nil {
		s.Fatal("Failed to get veth in address: ", err)
	}
	if err := expectCurlSuccessWithTimeout(ctx, 5*time.Second, portal, addrs.IPv4Addr.String()); err != nil {
		s.Fatal("Failed to curl http server in network namespace: ", err)
	}

	s.Log("Make service restart portal detector")
	if err := m.RecheckPortal(ctx); err != nil {
		s.Fatal("Failed to invoke RecheckPortal on shill: ", err)
	}

	device, err := m.WaitForDeviceByName(ctx, portal.VethOutName, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find veth device managed by Shill: ", err)
	}
	servicePath, err := device.WaitForSelectedService(ctx, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to get Service path: ", err)
	}
	service, err := shill.NewService(ctx, servicePath)
	if err != nil {
		s.Fatal("Failed to get Service: ", err)
	}

	s.Log("Polling until service state online")
	if err := pollServiceState(ctx, 15*time.Second, service, shillconst.ServiceStateOnline); err != nil {
		s.Fatal("Failed to poll service state: ", err)
	}
}

func expectCurlSuccessWithTimeout(ctx context.Context, timeout time.Duration, portal *env.Env, addr string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := []string{
			"curl",
			"-I",
			addr,
		}
		if out, err := portal.RunCommandInNetNS(ctx, cmd); err != nil {
			return err
		} else if !bytes.Contains(out, okHeader) {
			return errors.Errorf("unexpected curl result: got %s; want %s", out, okHeader)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: 500 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "failed to curl within timeout")
	}
	return nil
}

func pollServiceState(ctx context.Context, timeout time.Duration, service *shill.Service, expectedState string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		serviceProps, err := service.GetProperties(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get service properties")
		}

		state, err := serviceProps.GetString(shillconst.ServicePropertyState)
		if err != nil {
			return errors.Wrap(err, "failed to get service state")
		}
		if state != expectedState {
			return errors.Wrapf(err, "unexpected Service.State: %v", state)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: 500 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "failed to get service state within timeout")
	}
	return nil
}
