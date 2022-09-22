// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/common/network/ip"
	"chromiumos/tast/common/utils"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// NOTE: shill does not manage (i.e., run a dhcpcd on) the device with prefix "veth".
	// See kIgnoredDeviceNamePrefixes in http://cs/chromeos_public/src/platform2/shill/device_info.cc

	// VethPrefix is the prefix for the veth interface.
	VethPrefix = "vethA"
	// VethPeerPrefix is the prefix for the peer's veth interface.
	// Note: OpenWrt does not support setting the peer's veth interface name and
	// will always be "vethN", where N increments by 1 if vethN already exists,
	// starting at "veth0".
	VethPeerPrefix = "vethB"
)

// NewVethPair returns a veth pair for tests to use. Note that the caller is responsible to call ReleaseVethPair.
// The resolveIfaceNames parameter must be false for legacy routers.
func NewVethPair(ctx context.Context, ipr *ip.Runner, vethID int, resolveIfaceNames bool) (_, _ string, retErr error) {
	veth := fmt.Sprintf("%s%d", VethPrefix, vethID)
	vethPeer := fmt.Sprintf("%s%d", VethPeerPrefix, vethID)
	if err := ipr.AddLink(ctx, veth, "veth", "peer", "name", vethPeer); err != nil {
		return "", "", err
	}
	defer func() {
		if retErr != nil {
			if err := ipr.DeleteLink(ctx, veth); err != nil {
				testing.ContextLogf(ctx, "Failed to delete the veth %s while NewVethPair has failed", veth)
			}
		}
	}()

	if resolveIfaceNames {
		// The veth peer name may not have stuck depending on the ip implementation,
		// so resolve them.
		resolvedVeth, resolvedVethPeer, err := ResolveVethIfaceNames(ctx, ipr, veth)
		if err != nil {
			return "", "", errors.Wrap(err, "failed to resolve iface name of veth peer after adding link")
		}
		veth = resolvedVeth
		vethPeer = resolvedVethPeer
	}

	if err := ipr.SetLinkUp(ctx, veth); err != nil {
		return "", "", err
	}
	if err := ipr.SetLinkUp(ctx, vethPeer); err != nil {
		return "", "", err
	}
	return veth, vethPeer, nil
}

// ReleaseVethPair release the veth pair.
// Note that each side of the pair can be passed to this method, but the test should only call the method once for each pair.
// The resolveIfaceNames parameter must be false for legacy routers.
func ReleaseVethPair(ctx context.Context, ipr *ip.Runner, vethEnd string, resolveIfaceNames bool) error {
	var veth, vethPeer string
	if resolveIfaceNames {
		var err error
		veth, vethPeer, err = ResolveVethIfaceNames(ctx, ipr, vethEnd)
		if err != nil {
			return errors.Wrap(err, "failed to resolve iface name of veth peer after adding link")
		}
	} else {
		// If it is a peer side veth name, change it to another side.
		veth = vethEnd
		if strings.HasPrefix(veth, VethPeerPrefix) {
			veth = VethPrefix + veth[len(VethPeerPrefix):]
		}
		vethPeer = VethPeerPrefix + veth[len(VethPrefix):]
	}

	var firstErr error
	utils.CollectFirstErr(ctx, &firstErr, ipr.FlushIP(ctx, veth))
	utils.CollectFirstErr(ctx, &firstErr, ipr.SetLinkDown(ctx, veth))
	utils.CollectFirstErr(ctx, &firstErr, ipr.FlushIP(ctx, vethPeer))
	utils.CollectFirstErr(ctx, &firstErr, ipr.SetLinkDown(ctx, vethPeer))
	// Note that we only need to delete one side.
	utils.CollectFirstErr(ctx, &firstErr, ipr.DeleteLink(ctx, veth))
	return firstErr
}

// ResolveVethIfaceNames will return the veth and veth peer iface names, in that
// order, when given either the veth or veth peer iface name. This assumes that
// the veth iface starts with the VethPrefix and the veth peer iface does not.
//
// Note: This is not supported with legacy routers since its "ip link show"
// output format does show iface aliases.
func ResolveVethIfaceNames(ctx context.Context, ipr *ip.Runner, vethEnd string) (string, string, error) {
	vethEndAlias, err := ipr.IfaceAlias(ctx, vethEnd)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to get alias of iface %q", vethEnd)
	}
	if strings.HasPrefix(vethEnd, VethPrefix) {
		return vethEnd, vethEndAlias, nil
	}
	if !strings.HasPrefix(vethEndAlias, VethPrefix) {
		return "", "", errors.Wrapf(err, "unable to determine which end of the veth pair with interfaces %q and %q is the peer", vethEnd, vethEndAlias)
	}
	return vethEndAlias, vethEnd, nil
}

// BindVethToBridge binds the veth to bridge.
func BindVethToBridge(ctx context.Context, ipr *ip.Runner, veth, br string) error {
	return ipr.SetBridge(ctx, veth, br)
}

// UnbindVeth unbinds the veth to any other interface.
func UnbindVeth(ctx context.Context, ipr *ip.Runner, veth string) error {
	return ipr.UnsetBridge(ctx, veth)
}

// RemoveAllVethIfaces will delete any existing ifaces starting with VethPrefix.
// The veth peer ifaces do not need to be deleted manually, as they will be
// deleted along with the other end of the pair.
func RemoveAllVethIfaces(ctx context.Context, ipr *ip.Runner) error {
	if err := RemoveDevicesWithPrefix(ctx, ipr, VethPrefix); err != nil {
		return errors.Wrapf(err, "failed to remove all veth interfaces with prefix %q", VethPrefix)
	}
	return nil
}
