// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NetworkProviderChangeNames,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests name changing of NetworkProvider utility, example test for development and example purposes, not to be Merged",
		Contacts: []string{
			"jstanko@google.com",
			"cros-connectivity@google.com@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_prod_esim"},
		Fixture:      "cellular",
		Timeout:      9 * time.Minute,
	})
}

func NetworkProviderChangeNames(ctx context.Context, s *testing.State) {
	networkProvider, err := cellular.NewNetworkProvider(ctx, false)
	if err != nil {
		s.Fatal("Failed to create cellular network provider: ", err)
	}

	networkNames, err := networkProvider.NetworkNames(ctx)
	if err != nil {
		s.Fatal("Failed ot get network names: ", err)
	}
	for _, name := range networkNames {
		testing.ContextLogf(ctx, "Found cellular network: %s", name)
	}

	if err := rename(ctx, networkProvider, "CellularNetwork"); err != nil {
		s.Fatal("Failed to rename eSIM networks: ", err)
	}

	if err := rename(ctx, networkProvider, "MobileNetwork"); err != nil {
		s.Fatal("Failed to rename eSIM networks: ", err)
	}
}

func rename(ctx context.Context, networkProvider cellular.NetworkProvider, namePrefix string) error {
	testing.ContextLogf(ctx, "renaming eSIM profiles %s", namePrefix)
	if err := networkProvider.RenameESimProfiles(ctx, namePrefix); err != nil {
		return errors.Wrap(err, "failed to rename eSIM profiles")
	}

	networkNames, err := networkProvider.ESimNetworkNames(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get eSIM network names")
	}

	seen := make(map[string]bool)
	for _, name := range networkNames {
		testing.ContextLogf(ctx, "Found cellular network: %s", name)
		if seen[name] {
			return errors.Wrapf(err, "failed to check renamed eSIM, duplicate networks %s found", name)
		}
		if !strings.HasPrefix(name, namePrefix) {
			return errors.Wrapf(err, "renamed profile does not have prefix: %s", namePrefix)
		}
		seen[name] = true
	}

	return nil
}
