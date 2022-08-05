// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ConnectToBTPeers connects to btpeers with the given addresses and returns
// a list of chameleon.Chameleon controllers to use to interact with these
// btpeers' chameleond service.
//
// Each btpeer address must be resolvable from the DUT if this is used in a
// local test, or from the tast runner if this is used in a remote test.
// Addresses for local tests likely must be a direct IP address, since lab DUTs
// cannot usually resolve lab hostnames.
func ConnectToBTPeers(ctx context.Context, btpeerAddresses []string) ([]chameleon.Chameleond, error) {
	btpeers := make([]chameleon.Chameleond, len(btpeerAddresses))
	for i, addr := range btpeerAddresses {
		testing.ContextLogf(ctx, "Connecting to Chameleond btpeer #%d at %q", i+1, addr)
		btpeer, err := chameleon.NewChameleond(ctx, addr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to connect to Chameleond btpeer #%d at %q", i+1, addr)
		}
		btpeers[i] = btpeer
	}
	return btpeers, nil
}
