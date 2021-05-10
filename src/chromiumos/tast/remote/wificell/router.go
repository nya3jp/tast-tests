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
	remote_ip "chromiumos/tast/remote/network/ip"
	remote_iw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/log"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/ssh"
	"chromiumos/tast/timing"
)

// SuperRouter is a supertype of all routers. This is mostly used in tandem with a handler to see whether an object supports a subset of supported methods.
type SuperRouter interface {
	LegacyRouter
	AxRouter
	OpenwrtRouter
}

// AxRouter contains the funcionality that the ax testbed router should support.
type AxRouter interface {
	baseRouter
}

// LegacyRouter contains the functionality the legacy WiFi testing router should support.
type LegacyRouter interface {
	legacyOpenwrtShared
}

// OpenwrtRouter contains the functionality that the future openwrt testbed shall support.
type OpenwrtRouter interface {
	legacyOpenwrtShared
}

// baseRouter contains the basic functions shared across all routers.
type baseRouter interface {
	initialize(ctx, daemonCtx context.Context) error
	Close(ctx context.Context) error
}

// legacyOpenwrtShared contains the functionality shared between legacy routers and openwrt routers.
type legacyOpenwrtShared interface {
	baseRouter
	supportLogs
	supportCapture
	supportHostapd
	supportDHCP
	supportFrameSender
	supportIfaceManipuation
	supportVethBridgeBinding
	supportBridge
	supportVeth
}

type supportLogs interface {
	CollectLogs(ctx context.Context) error
}

type supportCapture interface {
	StartCapture(ctx context.Context, name string, ch int, freqOps []iw.SetFreqOption, pcapOps ...pcap.Option) (ret *pcap.Capturer, retErr error)
	StartRawCapturer(ctx context.Context, name, iface string, ops ...pcap.Option) (*pcap.Capturer, error)
	StopCapture(ctx context.Context, capturer *pcap.Capturer) error
	StopRawCapturer(ctx context.Context, capturer *pcap.Capturer) error
	ReserveForStopCapture(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc)
	ReserveForStopRawCapturer(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc)
}

type supportHostapd interface {
	StartHostapd(ctx context.Context, name string, conf *hostapd.Config) (_ *hostapd.Server, retErr error)
	StopHostapd(ctx context.Context, hs *hostapd.Server) error
	ReconfigureHostapd(ctx context.Context, hs *hostapd.Server, conf *hostapd.Config) (_ *hostapd.Server, retErr error)
}

type supportDHCP interface {
	StartDHCP(ctx context.Context, name, iface string, ipStart, ipEnd, serverIP, broadcastIP net.IP, mask net.IPMask) (_ *dhcp.Server, retErr error)
	StopDHCP(ctx context.Context, ds *dhcp.Server) error
}

type supportFrameSender interface {
	CloseFrameSender(ctx context.Context, s *framesender.Sender) error
	NewFrameSender(ctx context.Context, iface string) (ret *framesender.Sender, retErr error)
	ReserveForCloseFrameSender(ctx context.Context) (context.Context, context.CancelFunc)
}

type supportIfaceManipuation interface {
	SetAPIfaceDown(ctx context.Context, iface string) error
	MAC(ctx context.Context, iface string) (net.HardwareAddr, error)
}

type supportVethBridgeBinding interface {
	BindVethToBridge(ctx context.Context, veth, br string) error
	UnbindVeth(ctx context.Context, veth string) error
}
type supportBridge interface {
	NewBridge(ctx context.Context) (_ string, retErr error)
	ReleaseBridge(ctx context.Context, br string) error
}

type supportVeth interface {
	NewVethPair(ctx context.Context) (_, _ string, retErr error)
	ReleaseVethPair(ctx context.Context, veth string) error
}

// RouterType is an enum indicating what type of router style a router is.
type RouterType int

const (
	ax      RouterType = iota
	openwrt RouterType = iota
	legacy  RouterType = iota
)

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
func ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 5*time.Second)
}

// NewRouter connects to and initializes the router via SSH then returns the Router object.
// This method takes two context: ctx and daemonCtx, the first is the context for the New
// method and daemonCtx is for the spawned background daemons.
// After getting a Server instance, d, the caller should call r.Close() at the end, and use the
// shortened ctx (provided by d.ReserveForClose()) before r.Close() to reserve time for it to run.
func NewRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string, rtype RouterType) (SuperRouter, error) {
	ctx, st := timing.Start(ctx, "NewRouter")
	defer st.End()
	var r SuperRouter
	switch rtype {
	case legacy:
		r = &legacyRouterStruct{
			BaseRouterStruct: BaseRouterStruct{
				host:  host,
				name:  name,
				rtype: legacy,
			},
			phys:          make(map[int]*iw.Phy),
			availIfaces:   make(map[string]*iw.NetDev),
			busyIfaces:    make(map[string]*iw.NetDev),
			iwr:           remote_iw.NewRemoteRunner(host),
			ipr:           remote_ip.NewRemoteRunner(host),
			logCollectors: make(map[string]*log.Collector),
		}
	default:
		return nil, errors.Errorf("unexpected routerType, got %v", rtype)
	}
	shortCtx, cancel := ReserveForClose(ctx)
	defer cancel()
	if err := r.initialize(shortCtx, daemonCtx); err != nil {
		r.Close(ctx)
		return nil, err
	}
	return r, nil
}
