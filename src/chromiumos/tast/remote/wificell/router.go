// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"net"
	"time"

	"chromiumos/tast/common/network/ip"
	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/log"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/ssh"
	"chromiumos/tast/timing"
)

// RouterType is an enum indicating what type of router style a router is.
type RouterType int

const (
	legacy RouterType = iota
	ax
	openwrt
)

// BaseRouter contains the basic methods implemented across all routers.
type BaseRouter interface {
	// initialize prepares initial test AP state (e.g., initializing wiphy/wdev).
	initialize(ctx, daemonCtx context.Context) error
	// Close cleans the resource used by Router.
	Close(ctx context.Context) error
	// ReserveForClose returns a shortened ctx with cancel function.
	ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc)
	// GetRouterType
	GetRouterType() RouterType
}

// AxRouter contains the funcionality that the ax testbed router should support.
type AxRouter interface {
	BaseRouter
}

// LegacyRouter contains the functionality the legacy WiFi testing router should support.
type LegacyRouter interface {
	legacyOpenWrtShared
}

// OpenWrtRouter contains the functionality that the future OpenWrt testbed shall support.
type OpenWrtRouter interface {
	legacyOpenWrtShared
}

// legacyOpenWrtShared contains the functionality shared between legacy routers and openwrt routers.
type legacyOpenWrtShared interface {
	BaseRouter
	supportLogs
	supportCapture
	supportHostapd
	supportDhcp
	supportFrameSender
	supportIfaceManipuation
	supportVethBridgeBinding
	supportBridge
	supportVeth
}

// supportLogs shall be implemented if the router supports log collection.
type supportLogs interface {
	// CollectLogs downloads log files from router to OutDir.
	CollectLogs(ctx context.Context) error
}

// supportCapture shall be implemented if the router supports pcap capture.
type supportCapture interface {
	// StartCapture starts a packet captur
	StartCapture(ctx context.Context, name string, ch int, freqOps []iw.SetFreqOption, pcapOps ...pcap.Option) (ret *pcap.Capturer, retErr error)
	// StartRawCapturer starts a capturer on an existing interface on the router instead of a
	// monitor type interface.
	StartRawCapturer(ctx context.Context, name, iface string, ops ...pcap.Option) (*pcap.Capturer, error)
	// StopCapture stops the packet capturer and releases related resources.
	StopCapture(ctx context.Context, capturer *pcap.Capturer) error
	// StopRawCapturer stops the packet capturer (no extra resources to release).
	StopRawCapturer(ctx context.Context, capturer *pcap.Capturer) error
	// ReserveForStopCapture returns a shortened ctx with cancel function.
	ReserveForStopCapture(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc)
	// ReserveForStopRawCapturer returns a shortened ctx with cancel function.
	ReserveForStopRawCapturer(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc)
}

// supportHostapd shall be implemented if the router supports hostapd.
type supportHostapd interface {
	// StartHostapd starts the hostapd server.
	StartHostapd(ctx context.Context, name string, conf *hostapd.Config) (_ *hostapd.Server, retErr error)
	// StopHostapd stops the hostapd server.
	StopHostapd(ctx context.Context, hs *hostapd.Server) error
	// ReconfigureHostapd restarts the hostapd server with the new config. It preserves the interface and the name of the old hostapd server.
	ReconfigureHostapd(ctx context.Context, hs *hostapd.Server, conf *hostapd.Config) (_ *hostapd.Server, retErr error)
}

// supportDhcp shall be implemented if the router supports DHCP configuration.
type supportDhcp interface {
	// StartDHCP starts the DHCP server and configures the server IP.
	StartDHCP(ctx context.Context, name, iface string, ipStart, ipEnd, serverIP, broadcastIP net.IP, mask net.IPMask) (_ *dhcp.Server, retErr error)
	// StopDHCP stops the DHCP server and flushes the interface.
	StopDHCP(ctx context.Context, ds *dhcp.Server) error
}

// supportFrameSender shall be implemented if the router can send management frames.
type supportFrameSender interface {
	// CloseFrameSender closes frame sender and releases related resources.
	CloseFrameSender(ctx context.Context, s *framesender.Sender) error
	// NewFrameSender creates a frame sender object.
	NewFrameSender(ctx context.Context, iface string) (ret *framesender.Sender, retErr error)
	// ReserveForCloseFrameSender returns a shortened ctx with cancel function.
	ReserveForCloseFrameSender(ctx context.Context) (context.Context, context.CancelFunc)
}

// supportIfaceManipulation shall be implemented if the router can modify its iface configuration.
type supportIfaceManipuation interface {
	// SetAPIfaceDown brings down the interface that the APIface uses.
	SetAPIfaceDown(ctx context.Context, iface string) error
	// MAC returns the MAC address of iface on this router.
	MAC(ctx context.Context, iface string) (net.HardwareAddr, error)
}

// supportVethBridgeBinding shall be implemented if the router supports bridges, veths, and can bind bridges and veths.
type supportVethBridgeBinding interface {
	supportBridge
	supportVeth
	// BindVethToBridge binds the veth to bridge.
	BindVethToBridge(ctx context.Context, veth, br string) error
	// UnbindVeth unbinds the veth to any other interface.
	UnbindVeth(ctx context.Context, veth string) error
}

// supportBridge shall be implemented if the router supports network bridges.
type supportBridge interface {
	// NewBridge returns a bridge name for tests to use.
	NewBridge(ctx context.Context) (_ string, retErr error)
	// ReleaseBridge releases the bridge.
	ReleaseBridge(ctx context.Context, br string) error
}

type supportVeth interface {
	// NewVethPair returns a veth pair for tests to use.
	NewVethPair(ctx context.Context) (_, _ string, retErr error)
	// ReleaseVethPair release the veth pair.
	ReleaseVethPair(ctx context.Context, veth string) error
}

const (
	// Autotest may be used on these routers too, and if it failed to clean up, we may be out of space in /tmp.
	autotestWorkdirGlob = "/tmp/autotest-*"
	workingDir          = "/tmp/tast-test/"
)

const (
	// NOTE: shill does not manage (i.e., run a dhcpcd on) the device with prefix "veth".
	// See kIgnoredDeviceNamePrefixes in http://cs/chromeos_public/src/platform2/shill/device_info.cc
	vethPrefix     = "vethA"
	vethPeerPrefix = "vethB"
	bridgePrefix   = "tastbr"
)

// BaseRouterStruct contains the basic router variables.
type BaseRouterStruct struct {
	host  *ssh.Conn
	name  string
	rtype RouterType
}

// legacyRouterStruct is used to control the legacy wireless router and stores state of the router.
type legacyRouterStruct struct {
	BaseRouterStruct
	board         string
	phys          map[int]*iw.Phy       // map from phy idx to iw.Phy.
	availIfaces   map[string]*iw.NetDev // map from interface name to iw.NetDev.
	busyIfaces    map[string]*iw.NetDev // map from interface name to iw.NetDev.
	ifaceID       int
	bridgeID      int
	vethID        int
	iwr           *iw.Runner
	ipr           *ip.Runner
	logCollectors map[string]*log.Collector // map from log path to its collector.
}

// ReserveForClose returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.Close() to reserve time for it to run.
func (r *legacyRouterStruct) ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 5*time.Second)
}

// NewRouter connects to and initializes the router via SSH then returns the Router object.
// This method takes two context: ctx and daemonCtx, the first is the context for the New
// method and daemonCtx is for the spawned background daemons.
// After getting a Server instance, d, the caller should call r.Close() at the end, and use the
// shortened ctx (provided by d.ReserveForClose()) before r.Close() to reserve time for it to run.
func NewRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string) (BaseRouter, error) {
	ctx, st := timing.Start(ctx, "NewRouter")
	defer st.End()
	var rtype = legacy
	switch rtype {
	case legacy:
		return newLegacyRouter(ctx, daemonCtx, host, name)
	default:
		return nil, errors.Errorf("unexpected routerType, got %v", rtype)
	}

}
