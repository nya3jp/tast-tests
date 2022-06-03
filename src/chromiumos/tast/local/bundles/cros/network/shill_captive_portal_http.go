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
		Desc:     "TODO",
		Contacts: []string{"michaelrygiel@google.com", "cros-networking@google.com"},
		Attr:     []string{},
	})
}

// This is a copy of the original test to show how the new router can be used.
// I simply added a curl command to show that the http server was up and running
// in the router network namespace. Unsure if this should be committed, but
// wanted to show how to connect to http server.

func ShillCaptivePortalHTTP(ctx context.Context, s *testing.State) {
	testing.ContextLog(ctx, "Restarting shill")
	if err := resetshill.ResetShill(ctx); err != nil {
		s.Fatal("Failed to reset shill")
	}
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}
	testing.ContextLog(ctx, "Disabling portal detection on ethernet")
	if err := m.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "wifi,cellular"); err != nil {
		s.Fatal("Failed to disable portal detection on ethernet: ", err)
	}

	testing.ContextLog(ctx, "Setting up a netns for router")
	pool := subnet.NewPool()
	router, err := virtualnet.CreateRouterEnv(ctx, m, pool, virtualnet.EnvOptions{
		Priority:   5,
		NameSuffix: "",
		EnableDHCP: true,
		RAServer:   true,
		HTTPServer: true,
	})
	if err != nil {
		s.Fatal("Failed to create a router env: ", err)
	}
	defer router.Cleanup(ctx)

	// The curl command fails if the sleep is not here. My assumption is the
	// curl command starts before the http server goes online.
	testing.Sleep(ctx, 5*time.Second)

	s.Log("Attempt to connect to python http server")
	out, err := testexec.CommandContext(ctx, "ip", "netns", "exec", "netns-router", "curl", "-I", "http://127.0.0.1:8000").Output()
	if err != nil {
		s.Error("Curl command without auth failed: ", err)
	} else if !bytes.Contains(out, okHeader) {
		s.Errorf("Unexpected curl result without auth: got %s; want %s", out, okHeader)
	}

}
