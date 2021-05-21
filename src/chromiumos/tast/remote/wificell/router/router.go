// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package router

import (
	"context"
	"net"
	"time"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/ssh"
	"chromiumos/tast/timing"
)

// Type is an enum indicating what type of router style a router is.
type Type int

const (
	// LegacyT is the legacy router type.
	LegacyT Type = iota
	// AxT is the ax router type.
	AxT
	// OpenWrtT is the openwrt router type.
	OpenWrtT
)

// Base contains the basic methods implemented across all routers.
type Base interface {
	// Close cleans the resource used by Router.
	Close(ctx context.Context) error
	// GetRouterType
	GetRouterType() Type
}

// Ax contains the funcionality that the ax testbed router should support.
type Ax interface {
	Base
	GetRouterIP(ctx context.Context) (string, error)
	ApplyRouterSettings(ctx context.Context, settings []AxRouterConfigParam) error
}

// Legacy contains the functionality the legacy WiFi testing router should support.
type Legacy interface {
	LegacyOpenWrtShared
}

// OpenWrt contains the functionality that the future OpenWrt testbed shall support.
type OpenWrt interface {
	LegacyOpenWrtShared
}

// LegacyOpenWrtShared contains the functionality shared between legacy routers and openwrt routers.
type LegacyOpenWrtShared interface {
	Base
	SupportLogs
	SupportCapture
	SupportHostapd
	SupportDHCP
	SupportFrameSender
	SupportIfaceManipulation
	SupportVethBridgeBinding
	SupportBridge
	SupportVeth
}

// SupportLogs shall be implemented if the router supports log collection.
type SupportLogs interface {
	// CollectLogs downloads log files from router to OutDir.
	CollectLogs(ctx context.Context) error
}

// SupportCapture shall be implemented if the router supports pcap capture.
type SupportCapture interface {
	// StartCapture starts a packet capturer.
	StartCapture(ctx context.Context, name string, ch int, freqOps []iw.SetFreqOption, pcapOps ...pcap.Option) (*pcap.Capturer, error)
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

// SupportHostapd shall be implemented if the router supports hostapd.
type SupportHostapd interface {
	// StartHostapd starts the hostapd server.
	StartHostapd(ctx context.Context, name string, conf *hostapd.Config) (*hostapd.Server, error)
	// StopHostapd stops the hostapd server.
	StopHostapd(ctx context.Context, hs *hostapd.Server) error
	// ReconfigureHostapd restarts the hostapd server with the new config. It preserves the interface and the name of the old hostapd server.
	ReconfigureHostapd(ctx context.Context, hs *hostapd.Server, conf *hostapd.Config) (*hostapd.Server, error)
}

// SupportDHCP shall be implemented if the router supports DHCP configuration.
type SupportDHCP interface {
	// StartDHCP starts the DHCP server and configures the server IP.
	StartDHCP(ctx context.Context, name, iface string, ipStart, ipEnd, serverIP, broadcastIP net.IP, mask net.IPMask) (*dhcp.Server, error)
	// StopDHCP stops the DHCP server and flushes the interface.
	StopDHCP(ctx context.Context, ds *dhcp.Server) error
}

// SupportFrameSender shall be implemented if the router can send management frames.
type SupportFrameSender interface {
	// CloseFrameSender closes frame sender and releases related resources.
	CloseFrameSender(ctx context.Context, s *framesender.Sender) error
	// NewFrameSender creates a frame sender object.
	NewFrameSender(ctx context.Context, iface string) (*framesender.Sender, error)
	// ReserveForCloseFrameSender returns a shortened ctx with cancel function.
	ReserveForCloseFrameSender(ctx context.Context) (context.Context, context.CancelFunc)
}

// SupportIfaceManipulation shall be implemented if the router can modify its iface configuration.
type SupportIfaceManipulation interface {
	// SetAPIfaceDown brings down the interface that the APIface uses.
	SetAPIfaceDown(ctx context.Context, iface string) error
	// MAC returns the MAC address of iface on this router.
	MAC(ctx context.Context, iface string) (net.HardwareAddr, error)
}

// SupportVethBridgeBinding shall be implemented if the router supports bridges, veths, and can bind bridges and veths.
type SupportVethBridgeBinding interface {
	SupportBridge
	SupportVeth
	// BindVethToBridge binds the veth to bridge.
	BindVethToBridge(ctx context.Context, veth, br string) error
	// UnbindVeth unbinds the veth to any other interface.
	UnbindVeth(ctx context.Context, veth string) error
}

// SupportBridge shall be implemented if the router supports network bridges.
type SupportBridge interface {
	// NewBridge returns a bridge name for tests to use.
	NewBridge(ctx context.Context) (string, error)
	// ReleaseBridge releases the bridge.
	ReleaseBridge(ctx context.Context, br string) error
}

// SupportVeth shall be implemented if the router supports veth.
type SupportVeth interface {
	// NewVethPair returns a veth pair for tests to use.
	NewVethPair(ctx context.Context) (string, string, error)
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
	rtype Type
}

// ReserveForRouterClose returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.Close() to reserve time for it to run.
func ReserveForRouterClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 5*time.Second)
}

// NewRouter connects to and initializes the router via SSH then returns the Router object.
// This method takes two context: ctx and daemonCtx, the first is the context for the New
// method and daemonCtx is for the spawned background daemons.
// After getting a Server instance, d, the caller should call r.Close() at the end, and use the
// shortened ctx (provided by d.ReserveForClose()) before r.Close() to reserve time for it to run.
func NewRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string, rtype Type) (Base, error) {
	ctx, st := timing.Start(ctx, "NewRouter")
	defer st.End()
	switch rtype {
	case LegacyT:
		return newLegacyRouter(ctx, daemonCtx, host, name)
	case AxT:
		return newAxRouter(ctx, daemonCtx, host, name)
	default:
		return nil, errors.Errorf("unexpected routerType, got %v", rtype)
	}
}
