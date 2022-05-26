// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	resetshill "chromiumos/tast/local/bundles/cros/network/shill"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// This file is NOT for submission.

func init() {
	testing.AddTest(&testing.Test{
		Func:         Routing,
		Desc:         "TODO",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

func Routing(ctx context.Context, s *testing.State) {
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
	pool := virtualnet.NewSubnetPool()
	router, err := virtualnet.CreateRouterEnv(ctx, m, pool, virtualnet.EnvOptions{
		Priority:   5,
		NameSuffix: "",
		EnableDHCP: true,
		RAServer:   true,
	})
	if err != nil {
		s.Fatal("Failed to create a router env: ", err)
	}
	defer router.Cleanup(ctx)

	testing.Sleep(ctx, 5*time.Second)
}
