// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"strings"

	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// BTPeersVar is the name of the tast var that specifies a comma-separated
// list of btpeer host addresses.
const BTPeersVar = "btpeers"

// BTPeerTastVars are variables that BT fixtures may have. Can be used in fixture
// vars or test vars.
var BTPeerTastVars = []string{
	BTPeersVar,
}

// ConnectToBTPeers connects to the specified amount of btpeers and returns
// a list of chameleon.Chameleon controllers to use to interact with the
// btpeers' chameleond service.
//
// The "btpeers" test fixture var is parsed in-order for comma-separated btpeer
// host addresses. These host addresses must be resolvable from the dut, and
// since lab DUTs cannot usually resolve lab hostnames each address is likely
// required to be an IP.
//
// Only btpeers up to the required amount will be connected to. Once the
// required amount is reached successfully, any remaining known btpeers will
// be ignored. An error will be returned if there are not enough btpeers to meet
// the required amount or if it fails to connect to a btpeer.
func ConnectToBTPeers(ctx context.Context, btpeersVar string, requiredAmount int) ([]chameleon.Chameleond, error) {
	btpeers := make([]chameleon.Chameleond, 0)
	btpeerAddresses := strings.Split(btpeersVar, ",")
	for i, addr := range btpeerAddresses {
		if len(btpeers) >= requiredAmount {
			break
		}
		testing.ContextLogf(ctx, "Connecting to Chameleond btpeer #%d at %q", i+1, addr)
		btpeer, err := chameleon.NewChameleond(ctx, addr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to connect to Chameleond btpeer #%d at %q", i+1, addr)
		}
		btpeers = append(btpeers, btpeer)
	}
	if len(btpeers) != requiredAmount {
		return nil, errors.Errorf("failed to connect to required amount of btpeers: expected %d, got %d", requiredAmount, len(btpeers))
	}
	return btpeers, nil
}
