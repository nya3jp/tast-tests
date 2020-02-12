// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/shillscript"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillInitScriptsLoginStartShill,
		Desc:     "Test that shill init scripts perform as expected",
		Contacts: []string{"arowa@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func ShillInitScriptsLoginStartShill(ctx context.Context, s *testing.State) {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed locking the check network hook: ", err)
	}
	defer unlock()

	var env shillscript.TestEnv

	defer shillscript.TearDown(ctx, &env)

	if err := shillscript.SetUp(ctx, &env); err != nil {
		s.Fatal("Failed starting the test: ", err)
	}

	if err := testStartShill(ctx); err != nil {
		s.Fatal("Failed running testStartShill")
	}
}

// testStartShill tests all created path names during shill startup.
func testStartShill(ctx context.Context) error {
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	if err := shillscript.AssureIsDir("/run/shill"); err != nil {
		return errors.Wrap(err, "failed asserting that /run/shill is a directory")
	}

	if err := shillscript.AssureIsDir("/var/lib/dhcpcd"); err != nil {
		return errors.Wrap(err, "failed asserting that /var/lib/dhcpcd is a directory")
	}

	if err := shillscript.AssurePathOwner("/var/lib/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the user owner of the path /run/lib/dhcpcd is dhcp")
	}

	if err := shillscript.AssurePathGroup("/var/lib/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the group owner of the path /run/lib/dhcpcd is dhcp")
	}

	if err := shillscript.AssureIsDir("/run/dhcpcd"); err != nil {
		return errors.Wrap(err, "failed asserting that /run/dhcpcd is a directory")
	}

	if err := shillscript.AssurePathOwner("/run/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the user owner of the path /run/dhcpcd is dhcp")
	}

	if err := shillscript.AssurePathGroup("/run/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the group owner of the path /run/dhcpcd is dhcp")
	}

	return nil
}
