// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"

	"chromiumos/tast/common/network/ip"
	"chromiumos/tast/common/utils"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// BridgePrefix is the prefix for the bridge interface.
const BridgePrefix = "tastbr"

// NewBridge returns a bridge name for tests to use. Note that the caller is responsible to call ReleaseBridge.
func NewBridge(ctx context.Context, ipr *ip.Runner, bridgeID int) (string, error) {
	br := fmt.Sprintf("%s%d", BridgePrefix, bridgeID)
	if err := ipr.AddLink(ctx, br, "bridge"); err != nil {
		return "", err
	}
	if err := ipr.SetLinkUp(ctx, br); err != nil {
		if err := ipr.DeleteLink(ctx, br); err != nil {
			testing.ContextLog(ctx, "Failed to delete bridge while NewBridge has failed: ", err)
		}
		return "", err
	}
	return br, nil
}

// ReleaseBridge releases the bridge.
func ReleaseBridge(ctx context.Context, ipr *ip.Runner, br string) error {
	var firstErr error
	utils.CollectFirstErr(ctx, &firstErr, ipr.FlushIP(ctx, br))
	utils.CollectFirstErr(ctx, &firstErr, ipr.SetLinkDown(ctx, br))
	utils.CollectFirstErr(ctx, &firstErr, ipr.DeleteLink(ctx, br))
	return firstErr
}

// RemoveAllBridgeIfaces deletes any existing ifaces starting with BridgePrefix.
func RemoveAllBridgeIfaces(ctx context.Context, ipr *ip.Runner) error {
	if err := RemoveDevicesWithPrefix(ctx, ipr, BridgePrefix); err != nil {
		return errors.Wrapf(err, "failed to remove all bridge interfaces with prefix %q", BridgePrefix)
	}
	return nil
}
