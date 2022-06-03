// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	resetshill "chromiumos/tast/local/bundles/cros/network/shill"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

var (
	okHeader = []byte("HTTP/1.0 200")
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCaptivePortalHTTP,
		Desc:     "This test creates a virtual ethernet pair to a separate network namespace. The network namespace contains a DNS server and a http server to mock captive portal calls to test the service state of the veth device",
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

	testing.ContextLog(ctx, "Setting up a netns for captive portal")
	pool := subnet.NewPool()
	portal, err := virtualnet.CreateCaptivePortalEnv(ctx, m, pool, virtualnet.CaptivePortalOptions{
		Priority:         5,
		NameSuffix:       "",
		AddressToForceIP: "www.gstatic.com",
	})
	if err != nil {
		s.Fatal("Failed to create a portal env: ", err)
	}
	defer portal.Cleanup(ctx)

	s.Log("Connecting to http server")
	if err := expectCurlSuccessWithTimeout(ctx, 5*time.Second, portal.NetNSName, "192.168.101.1:80"); err != nil {
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

func expectCurlSuccessWithTimeout(ctx context.Context, timeout time.Duration, netns, addr string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "ip", "netns", "exec", netns, "curl", "-I", addr).Output()
		if err != nil {
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
