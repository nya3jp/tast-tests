// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/network/routing"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/testing"
)

const (
	primaryIPv4Only = "ipv4-only"
	primaryIPv6Only = "ipv6-only"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutingFallthrough,
		Desc:         "Verify the fall-through behavior for one IP family when the primary network is only configured with another family",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Params: []testing.Param{{
			Name: "ipv4_only_primary",
			Val:  primaryIPv4Only,
		}, {
			Name: "ipv6_only_primary",
			Val:  primaryIPv6Only,
		}},
	})
}

// RoutingFallthrough sets up the test network as a single-stack primary
// network, and then verifies the behavior on the secondary base network.
func RoutingFallthrough(ctx context.Context, s *testing.State) {
	testEnv := routing.NewTestEnv()
	if err := testEnv.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up routing test env: ", err)
	}
	defer func() {
		if err := testEnv.TearDown(ctx); err != nil {
			s.Error("Failed to tear down routing test env: ", err)
		}
	}()

	primaryFamily := s.Param().(string)

	testNetworkOpts := virtualnet.EnvOptions{
		Priority:   routing.HighPriority,
		NameSuffix: routing.TestSuffix,
		EnableDHCP: primaryFamily == primaryIPv4Only,
		RAServer:   primaryFamily == primaryIPv6Only,
	}
	if err := testEnv.CreateNetworkEnvForTest(ctx, testNetworkOpts); err != nil {
		s.Fatal("Failed to create network for test: ", err)
	}
	if err := testEnv.WaitForServiceOnline(ctx, testEnv.TestService); err != nil {
		s.Fatal("Failed to wait for the test service online: ", err)
	}
	if errs := testEnv.VerifyTestNetwork(ctx, routing.VerifyOptions{
		IPv4:      primaryFamily == primaryIPv4Only,
		IPv6:      primaryFamily == primaryIPv6Only,
		IsPrimary: true,
		Timeout:   5 * time.Second,
	}); len(errs) != 0 {
		for _, err := range errs {
			s.Errorf("Failed to verify connectivity to the %s test network: %v", primaryFamily, err)
		}
	}

	// The base network has both IPv4 and IPv6. Verify its behavior as a
	// non-primary network.
	if errs := testEnv.VerifyBaseNetwork(ctx, routing.VerifyOptions{
		IPv4:          true,
		IPv6:          true,
		IsPrimary:     false,
		IsHighestIPv6: primaryFamily == primaryIPv4Only,
		Timeout:       0,
	}); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed to verify connectivity to the base network: ", err)
		}
	}

	// TODO(b/234553227): Test metered networks when we implement that properly.
}
