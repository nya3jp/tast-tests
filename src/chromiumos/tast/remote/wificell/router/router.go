// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package router

import (
	"context"
	"net"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/remote/wificell/router/axrouter"
	"chromiumos/tast/remote/wificell/router/common"
)

// Base contains the basic methods implemented across all routers.
type Base interface {
	// Close cleans the resource used by Router.
	Close(ctx context.Context) error
	// RouterType returns the router type.
	RouterType() common.Type
}

// Ax contains the funcionality that the ax testbed router should support.
type Ax interface {
	Base
	// RouterIP gets the router's IP address.
	RouterIP(ctx context.Context) (string, error)
	// ApplyRouterSettings takes in the router config parameters, stages them, and then restarts the wireless service to have the changes realized on the router.
	ApplyRouterSettings(ctx context.Context, cfg *axrouter.Config) error
	// SaveConfiguration snapshots the router's configuration from the RAM from the stdout of "nvram show" into a string.
	SaveConfiguration(ctx context.Context) (string, error)
	// RestoreConfiguration takes a saved router configuration map, loads it into the RAM and updates the router.
	RestoreConfiguration(ctx context.Context, recoveryMap map[string]axrouter.ConfigParam) error
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
