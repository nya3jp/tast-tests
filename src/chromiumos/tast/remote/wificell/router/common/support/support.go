// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package support

import (
	"context"
	"net"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
)

// RouterType is an enum indicating what type of router style a router is.
type RouterType int

const (
	// LegacyT is the legacy router type.
	LegacyT RouterType = iota
	// AxT is the ax router type.
	AxT
	// OpenWrtT is the openwrt router type.
	OpenWrtT
)

// Base contains the basic methods that must be implemented across all routers.
type Base interface {
	// Close cleans the resource used by Router.
	Close(ctx context.Context) error
	// RouterType returns the router type.
	RouterType() RouterType
}

// Logs shall be implemented if the router supports log collection.
type Logs interface {
	Base
	// CollectLogs downloads log files from router to OutDir.
	CollectLogs(ctx context.Context) error
}

// Capture shall be implemented if the router supports pcap capture.
type Capture interface {
	Base
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

// Hostapd shall be implemented if the router supports hostapd.
type Hostapd interface {
	Base
	// StartHostapd starts the hostapd server.
	StartHostapd(ctx context.Context, name string, conf *hostapd.Config) (*hostapd.Server, error)
	// StopHostapd stops the hostapd server.
	StopHostapd(ctx context.Context, hs *hostapd.Server) error
	// ReconfigureHostapd restarts the hostapd server with the new config. It preserves the interface and the name of the old hostapd server.
	ReconfigureHostapd(ctx context.Context, hs *hostapd.Server, conf *hostapd.Config) (*hostapd.Server, error)
}

// DHCP shall be implemented if the router supports DHCP configuration.
type DHCP interface {
	Base
	// StartDHCP starts the DHCP server and configures the server IP.
	StartDHCP(ctx context.Context, name, iface string, ipStart, ipEnd, serverIP, broadcastIP net.IP, mask net.IPMask) (*dhcp.Server, error)
	// StopDHCP stops the DHCP server and flushes the interface.
	StopDHCP(ctx context.Context, ds *dhcp.Server) error
}

// FrameSender shall be implemented if the router can send management frames.
type FrameSender interface {
	Base
	// CloseFrameSender closes frame sender and releases related resources.
	CloseFrameSender(ctx context.Context, s *framesender.Sender) error
	// NewFrameSender creates a frame sender object.
	NewFrameSender(ctx context.Context, iface string) (*framesender.Sender, error)
	// ReserveForCloseFrameSender returns a shortened ctx with cancel function.
	ReserveForCloseFrameSender(ctx context.Context) (context.Context, context.CancelFunc)
}

// IfaceManipulation shall be implemented if the router can modify its iface configuration.
type IfaceManipulation interface {
	Base
	// SetAPIfaceDown brings down the interface that the APIface uses.
	SetAPIfaceDown(ctx context.Context, iface string) error
	// MAC returns the MAC address of iface on this router.
	MAC(ctx context.Context, iface string) (net.HardwareAddr, error)
}

// Bridge shall be implemented if the router supports network bridges.
type Bridge interface {
	Base
	// NewBridge returns a bridge name for tests to use.
	NewBridge(ctx context.Context) (string, error)
	// ReleaseBridge releases the bridge.
	ReleaseBridge(ctx context.Context, br string) error
}

// Veth shall be implemented if the router supports veth.
type Veth interface {
	Base
	// NewVethPair returns a veth pair for tests to use.
	NewVethPair(ctx context.Context) (string, string, error)
	// ReleaseVethPair release the veth pair.
	ReleaseVethPair(ctx context.Context, veth string) error
}

// VethBridgeBinding shall be implemented if the router supports bridges, veths, and can bind bridges and veths.
type VethBridgeBinding interface {
	Base
	Bridge
	Veth
	// BindVethToBridge binds the veth to bridge.
	BindVethToBridge(ctx context.Context, veth, br string) error
	// UnbindVeth unbinds the veth to any other interface.
	UnbindVeth(ctx context.Context, veth string) error
}

// WithLogs casts a router to one that supports Logs while confirming it has the support.
func WithLogs(br Base) (Logs, error) {
	r, ok := br.(Logs)
	if !ok {
		return nil, errors.New("router does not support Logs")
	}
	return r, nil
}

// WithCapture casts a router to one that supports Capture while confirming it has the support.
func WithCapture(br Base) (Capture, error) {
	r, ok := br.(Capture)
	if !ok {
		return nil, errors.New("router does not support Capture")
	}
	return r, nil
}

// WithHostapd casts a router to one that supports Hostapd while confirming it has the support.
func WithHostapd(br Base) (Hostapd, error) {
	r, ok := br.(Hostapd)
	if !ok {
		return nil, errors.New("router does not support Hostapd")
	}
	return r, nil
}

// WithDHCP casts a router to one that supports DHCP while confirming it has the support.
func WithDHCP(br Base) (DHCP, error) {
	r, ok := br.(DHCP)
	if !ok {
		return nil, errors.New("router does not support DHCP")
	}
	return r, nil
}

// WithFrameSender casts a router to one that supports FrameSender while confirming it has the support.
func WithFrameSender(br Base) (FrameSender, error) {
	r, ok := br.(FrameSender)
	if !ok {
		return nil, errors.New("router does not support FrameSender")
	}
	return r, nil
}

// WithIfaceManipulation casts a router to one that supports IfaceManipulation while confirming it has the support.
func WithIfaceManipulation(br Base) (IfaceManipulation, error) {
	r, ok := br.(IfaceManipulation)
	if !ok {
		return nil, errors.New("router does not support IfaceManipulation")
	}
	return r, nil
}

// WithBridge casts a router to one that supports Bridge while confirming it has the support.
func WithBridge(br Base) (Bridge, error) {
	r, ok := br.(Bridge)
	if !ok {
		return nil, errors.New("router does not support Bridge")
	}
	return r, nil
}

// WithVeth casts a router to one that supports Veth while confirming it has the support.
func WithVeth(br Base) (Veth, error) {
	r, ok := br.(Veth)
	if !ok {
		return nil, errors.New("router does not support Veth")
	}
	return r, nil
}

// WithVethBridgeBinding casts a router to one that supports VethBridgeBinding while confirming it has the support.
func WithVethBridgeBinding(br Base) (VethBridgeBinding, error) {
	r, ok := br.(VethBridgeBinding)
	if !ok {
		return nil, errors.New("router does not support VethBridgeBinding")
	}
	return r, nil
}
