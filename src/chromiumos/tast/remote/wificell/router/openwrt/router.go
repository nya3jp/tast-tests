// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package openwrt

import (
	"context"
	"net"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/ssh"
)

// Router controls an OpenWrt router and stores the router state.
type Router struct {
	host       *ssh.Conn
	name       string
	routerType support.RouterType
}

// NewRouter prepares initial test AP state (e.g., initializing wiphy/wdev).
// ctx is the deadline for the step and daemonCtx is the lifetime for background
// daemons.
func NewRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string) (*Router, error) {
	r := &Router{
		host:       host,
		name:       name,
		routerType: support.OpenWrtT,
	}
	// TODO
	return r, nil
}

// Close cleans the resource used by Router.
func (r Router) Close(ctx context.Context) error {
	// TODO
	panic("implement me")
}

// RouterType returns the router type.
func (r Router) RouterType() support.RouterType {
	return r.routerType
}

// RouterTypeName returns the human-readable name this Router's RouterType
func (r Router) RouterTypeName() string {
	return support.RouterTypeName(r.routerType)
}

// RouterName returns the name of the managed router device.
func (r *Router) RouterName() string {
	return r.name
}

// CollectLogs downloads log files from router to OutDir.
func (r Router) CollectLogs(ctx context.Context) error {
	panic("implement me")
}

// StartHostapd starts the hostapd server.
func (r Router) StartHostapd(ctx context.Context, name string, conf *hostapd.Config) (*hostapd.Server, error) {
	panic("implement me")
}

// StopHostapd stops the hostapd server.
func (r Router) StopHostapd(ctx context.Context, hs *hostapd.Server) error {
	panic("implement me")
}

// ReconfigureHostapd restarts the hostapd server with the new config. It preserves the interface and the name of the old hostapd server.
func (r Router) ReconfigureHostapd(ctx context.Context, hs *hostapd.Server, conf *hostapd.Config) (*hostapd.Server, error) {
	panic("implement me")
}

// StartDHCP starts the DHCP server and configures the server IP.
func (r Router) StartDHCP(ctx context.Context, name, iface string, ipStart, ipEnd, serverIP, broadcastIP net.IP, mask net.IPMask) (*dhcp.Server, error) {
	panic("implement me")
}

// StopDHCP stops the DHCP server and flushes the interface.
func (r Router) StopDHCP(ctx context.Context, ds *dhcp.Server) error {
	panic("implement me")
}

// StartCapture starts a packet capturer.
func (r Router) StartCapture(ctx context.Context, name string, ch int, freqOps []iw.SetFreqOption, pcapOps ...pcap.Option) (*pcap.Capturer, error) {
	panic("implement me")
}

// StartRawCapturer starts a capturer on an existing interface on the router instead of a
// monitor type interface.
func (r Router) StartRawCapturer(ctx context.Context, name, iface string, ops ...pcap.Option) (*pcap.Capturer, error) {
	panic("implement me")
}

// StopCapture stops the packet capturer and releases related resources.
func (r Router) StopCapture(ctx context.Context, capturer *pcap.Capturer) error {
	panic("implement me")
}

// StopRawCapturer stops the packet capturer (no extra resources to release).
func (r Router) StopRawCapturer(ctx context.Context, capturer *pcap.Capturer) error {
	panic("implement me")
}

// ReserveForStopCapture returns a shortened ctx with cancel function.
func (r Router) ReserveForStopCapture(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc) {
	panic("implement me")
}

// ReserveForStopRawCapturer returns a shortened ctx with cancel function.
func (r Router) ReserveForStopRawCapturer(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc) {
	panic("implement me")
}
