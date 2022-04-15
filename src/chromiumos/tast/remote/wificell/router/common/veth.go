package common

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/common/network/ip"
	"chromiumos/tast/remote/wificell/wifiutil"
	"chromiumos/tast/testing"
)

const (
	// NOTE: shill does not manage (i.e., run a dhcpcd on) the device with prefix "veth".
	// See kIgnoredDeviceNamePrefixes in http://cs/chromeos_public/src/platform2/shill/device_info.cc

	// VethPrefix is the prefix for the veth interface.
	VethPrefix = "vethA"
	// VethPeerPrefix is the prefix for the peer's veth interface.
	VethPeerPrefix = "vethB"
)

// NewVethPair returns a veth pair for tests to use. Note that the caller is responsible to call ReleaseVethPair.
func NewVethPair(ctx context.Context, ipr *ip.Runner, vethID int) (_, _ string, retErr error) {
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
func ReleaseVethPair(ctx context.Context, ipr *ip.Runner, veth string) error {
	// If it is a peer side veth name, change it to another side.
	if strings.HasPrefix(veth, VethPeerPrefix) {
		veth = VethPrefix + veth[len(VethPeerPrefix):]
	}
	vethPeer := VethPeerPrefix + veth[len(VethPrefix):]

	var firstErr error
	wifiutil.CollectFirstErr(ctx, &firstErr, ipr.FlushIP(ctx, veth))
	wifiutil.CollectFirstErr(ctx, &firstErr, ipr.SetLinkDown(ctx, veth))
	wifiutil.CollectFirstErr(ctx, &firstErr, ipr.FlushIP(ctx, vethPeer))
	wifiutil.CollectFirstErr(ctx, &firstErr, ipr.SetLinkDown(ctx, vethPeer))
	// Note that we only need to delete one side.
	wifiutil.CollectFirstErr(ctx, &firstErr, ipr.DeleteLink(ctx, veth))
	return firstErr
}

// BindVethToBridge binds the veth to bridge.
func BindVethToBridge(ctx context.Context, ipr *ip.Runner, veth, br string) error {
	return ipr.SetBridge(ctx, veth, br)
}

// UnbindVeth unbinds the veth to any other interface.
func UnbindVeth(ctx context.Context, ipr *ip.Runner, veth string) error {
	return ipr.UnsetBridge(ctx, veth)
}
