// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package support

import (
	"context"
	"net"
	"strings"

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

// RouterTypeName returns the human-readable name for a given RouterType
func RouterTypeName(rt RouterType) string {
	switch rt {
	case LegacyT:
		return "Legacy"
	case AxT:
		return "CiscoAX"
	case OpenWrtT:
		return "OpenWrt"
	default:
		return "Unknown"
	}
}

// Router contains the basic methods that must be implemented across all routers.
type Router interface {
	// Close cleans the resource used by Router.
	Close(ctx context.Context) error
	// RouterName returns the name of the managed router device.
	RouterName() string
	// RouterType returns the router type.
	RouterType() RouterType
	// RouterTypeName returns the human-readable name this Router's RouterType
	RouterTypeName() string
}

// Logs shall be implemented if the router supports log collection.
type Logs interface {
	Router
	// CollectLogs downloads log files from router to OutDir.
	CollectLogs(ctx context.Context) error
}

// Capture shall be implemented if the router supports pcap capture.
type Capture interface {
	Router
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
	Router
	// StartHostapd starts the hostapd server.
	StartHostapd(ctx context.Context, name string, conf *hostapd.Config) (*hostapd.Server, error)
	// StopHostapd stops the hostapd server.
	StopHostapd(ctx context.Context, hs *hostapd.Server) error
	// ReconfigureHostapd restarts the hostapd server with the new config. It preserves the interface and the name of the old hostapd server.
	ReconfigureHostapd(ctx context.Context, hs *hostapd.Server, conf *hostapd.Config) (*hostapd.Server, error)
}

// DHCP shall be implemented if the router supports DHCP configuration.
type DHCP interface {
	Router
	// StartDHCP starts the DHCP server and configures the server IP.
	StartDHCP(ctx context.Context, name, iface string, ipStart, ipEnd, serverIP, broadcastIP net.IP, mask net.IPMask) (*dhcp.Server, error)
	// StopDHCP stops the DHCP server and flushes the interface.
	StopDHCP(ctx context.Context, ds *dhcp.Server) error
}

// FrameSender shall be implemented if the router can send management frames.
type FrameSender interface {
	Router
	// CloseFrameSender closes frame sender and releases related resources.
	CloseFrameSender(ctx context.Context, s *framesender.Sender) error
	// NewFrameSender creates a frame sender object.
	NewFrameSender(ctx context.Context, iface string) (*framesender.Sender, error)
	// ReserveForCloseFrameSender returns a shortened ctx with cancel function.
	ReserveForCloseFrameSender(ctx context.Context) (context.Context, context.CancelFunc)
}

// IfaceManipulation shall be implemented if the router can modify its iface configuration.
type IfaceManipulation interface {
	Router
	// SetAPIfaceDown brings down the interface that the APIface uses.
	SetAPIfaceDown(ctx context.Context, iface string) error
	// MAC returns the MAC address of iface on this router.
	MAC(ctx context.Context, iface string) (net.HardwareAddr, error)
}

// Bridge shall be implemented if the router supports network bridges.
type Bridge interface {
	Router
	// NewBridge returns a bridge name for tests to use.
	NewBridge(ctx context.Context) (string, error)
	// ReleaseBridge releases the bridge.
	ReleaseBridge(ctx context.Context, br string) error
}

// Veth shall be implemented if the router supports veth.
type Veth interface {
	Router
	// NewVethPair returns a veth pair for tests to use.
	NewVethPair(ctx context.Context) (string, string, error)
	// ReleaseVethPair release the veth pair.
	ReleaseVethPair(ctx context.Context, veth string) error
}

// VethBridgeBinding shall be implemented if the router supports bridges, veths, and can bind bridges and veths.
type VethBridgeBinding interface {
	Router
	Bridge
	Veth
	// BindVethToBridge binds the veth to bridge.
	BindVethToBridge(ctx context.Context, veth, br string) error
	// UnbindVeth unbinds the veth to any other interface.
	UnbindVeth(ctx context.Context, veth string) error
}

// Validate confirms that the router has all the desired support, and will
// return a friendly error message if it finds that it is not.
//
// The name of the support is name of the support interface, case-insensitive.
func Validate(r Router, allRequiredSupport []string) error {
	for _, rs := range allRequiredSupport {
		var err error
		if strings.EqualFold(rs, "Logs") {
			_, err = WithLogs(r)
		} else if strings.EqualFold(rs, "Capture") {
			_, err = WithCapture(r)
		} else if strings.EqualFold(rs, "Hostapd") {
			_, err = WithHostapd(r)
		} else if strings.EqualFold(rs, "DHCP") {
			_, err = WithDHCP(r)
		} else if strings.EqualFold(rs, "FrameSender") {
			_, err = WithFrameSender(r)
		} else if strings.EqualFold(rs, "IfaceManipulation") {
			_, err = WithIfaceManipulation(r)
		} else if strings.EqualFold(rs, "Bridge") {
			_, err = WithBridge(r)
		} else if strings.EqualFold(rs, "Veth") {
			_, err = WithVeth(r)
		} else if strings.EqualFold(rs, "VethBridgeBinding") {
			_, err = WithVethBridgeBinding(r)
		} else {
			err = errors.Errorf("unknown required router support %q", rs)
		}
		if err != nil {
			return errors.Wrap(err, "router missing requires support")
		}
	}
	return nil
}

// WithLogs casts a router to one that supports Logs while confirming it has the support.
func WithLogs(r Router) (Logs, error) {
	routerWithSupport, ok := r.(Logs)
	if !ok {
		return nil, errors.Errorf("router type %q does not support Logs", r.RouterTypeName())
	}
	return routerWithSupport, nil
}

// WithCapture casts a router to one that supports Capture while confirming it has the support.
func WithCapture(r Router) (Capture, error) {
	routerWithSupport, ok := r.(Capture)
	if !ok {
		return nil, errors.Errorf("router type %q does not support Capture", r.RouterTypeName())
	}
	return routerWithSupport, nil
}

// WithHostapd casts a router to one that supports Hostapd while confirming it has the support.
func WithHostapd(r Router) (Hostapd, error) {
	routerWithSupport, ok := r.(Hostapd)
	if !ok {
		return nil, errors.Errorf("router type %q does not support Hostapd", r.RouterTypeName())
	}
	return routerWithSupport, nil
}

// WithDHCP casts a router to one that supports DHCP while confirming it has the support.
func WithDHCP(r Router) (DHCP, error) {
	routerWithSupport, ok := r.(DHCP)
	if !ok {
		return nil, errors.Errorf("router type %q does not support DHCP", r.RouterTypeName())
	}
	return routerWithSupport, nil
}

// WithFrameSender casts a router to one that supports FrameSender while confirming it has the support.
func WithFrameSender(r Router) (FrameSender, error) {
	routerWithSupport, ok := r.(FrameSender)
	if !ok {
		return nil, errors.Errorf("router type %q does not support FrameSender", r.RouterTypeName())
	}
	return routerWithSupport, nil
}

// WithIfaceManipulation casts a router to one that supports IfaceManipulation while confirming it has the support.
func WithIfaceManipulation(r Router) (IfaceManipulation, error) {
	routerWithSupport, ok := r.(IfaceManipulation)
	if !ok {
		return nil, errors.Errorf("router type %q does not support IfaceManipulation", r.RouterTypeName())
	}
	return routerWithSupport, nil
}

// WithBridge casts a router to one that supports Bridge while confirming it has the support.
func WithBridge(r Router) (Bridge, error) {
	routerWithSupport, ok := r.(Bridge)
	if !ok {
		return nil, errors.Errorf("router type %q does not support Bridge", r.RouterTypeName())
	}
	return routerWithSupport, nil
}

// WithVeth casts a router to one that supports Veth while confirming it has the support.
func WithVeth(r Router) (Veth, error) {
	routerWithSupport, ok := r.(Veth)
	if !ok {
		return nil, errors.Errorf("router type %q does not support Veth", r.RouterTypeName())
	}
	return routerWithSupport, nil
}

// WithVethBridgeBinding casts a router to one that supports VethBridgeBinding while confirming it has the support.
func WithVethBridgeBinding(r Router) (VethBridgeBinding, error) {
	routerWithSupport, ok := r.(VethBridgeBinding)
	if !ok {
		return nil, errors.Errorf("router type %q does not support VethBridgeBinding", r.RouterTypeName())
	}
	return routerWithSupport, nil
}
