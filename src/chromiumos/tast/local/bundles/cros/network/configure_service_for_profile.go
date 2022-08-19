// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConfigureServiceForProfile,
		Desc: "Test ConfigureServiceForProfile D-Bus method",
		Contacts: []string{
			"matthewmwang@chromium.org",
		},
		// b:238260020 - disable aged (>1y) unpromoted informational tests
		// Attr:    []string{"group:mainline", "informational"},
		Fixture: "shillReset",
	})
}

func ConfigureServiceForProfile(ctx context.Context, s *testing.State) {
	const (
		objectPath = shillconst.DefaultProfileObjectPath
	)

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	props := map[string]interface{}{
		shillconst.ServicePropertyType: "ethernet",
		shillconst.ServicePropertyStaticIPConfig: map[string]interface{}{
			shillconst.IPConfigPropertyNameServers: []string{"8.8.8.8"},
		},
	}
	_, err = manager.ConfigureServiceForProfile(ctx, objectPath, props)
	if err != nil {
		s.Fatal("Unable to configure service: ", err)
	}

	// Restart shill to ensure that configurations persist across reboot.
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}
	manager, err = shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	if _, err := manager.WaitForServiceProperties(ctx, props, 8*time.Second); err != nil {
		s.Fatal("Service not found: ", err)
	}
}
