// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"net"
	"net/http"

	"chromiumos/tast/common/utils"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/hostapd"
	httpServer "chromiumos/tast/remote/wificell/http"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// busySubnet records the used subnet indexes.
// It is a global variable because the usage of subnet is not limited in a certain router or ap.
var busySubnet = make(map[byte]struct{})

// reserveSubnetIdx finds a free subnet index and reserves it.
func reserveSubnetIdx() (byte, error) {
	for i := byte(0); i <= 255; i++ {
		if _, ok := busySubnet[i]; ok {
			continue
		}
		busySubnet[i] = struct{}{}
		return i, nil
	}
	return 0, errors.New("subnet index exhausted")
}

// freeSubnetIdx marks the subnet index as unused.
func freeSubnetIdx(i byte) {
	delete(busySubnet, i)
}

type supportedRouter interface {
	support.Hostapd
	support.DHCP
	support.HTTP
}

const (
	dnsPort         = 53
	httpPort        = 80
	httpRedirectURL = "http://example.com/"
)

// APIface is the handle object of an instance of hostapd service managed by a router.
// It is comprised of a hostapd and a dhcpd. The DHCP server is assigned with the subnet
// 192.168.$subnetIdx.0/24.
type APIface struct {
	router    supportedRouter
	name      string
	iface     string
	subnetIdx byte

	hostapd    *hostapd.Server
	dhcpd      *dhcp.Server
	httpServer *httpServer.Server

	stopped bool // true if Stop() is called. Used to avoid Stop() being called twice.
}

// Config returns the config of hostapd.
// NOTE: Caller should not modify the returned object.
func (h *APIface) Config() *hostapd.Config {
	return h.hostapd.Config()
}

// subnetIP returns 192.168.$subnetIdx.$suffix IP.
func (h *APIface) subnetIP(suffix byte) net.IP {
	return net.IPv4(192, 168, h.subnetIdx, suffix)
}

// mask returns the mask of the working subnet.
func (h *APIface) mask() net.IPMask {
	return net.IPv4Mask(255, 255, 255, 0)
}

// broadcastIP returns the broadcast IP of working subnet.
func (h *APIface) broadcastIP() net.IP {
	return h.subnetIP(255)
}

// ServerIP returns the IP of router in the subnet of WiFi.
func (h *APIface) ServerIP() net.IP {
	return h.subnetIP(254)
}

// Interface returns the interface the service runs on.
func (h *APIface) Interface() string {
	return h.iface
}

// ServerSubnet returns the subnet whose ip has been masked.
func (h *APIface) ServerSubnet() *net.IPNet {
	mask := h.mask()
	ip := h.ServerIP().Mask(mask)
	return &net.IPNet{IP: ip, Mask: mask}
}

// StartAPIface starts the service.
// After started, the caller should call h.Stop() at the end, and use the shortened ctx
// (provided by h.ReserveForStop()) before h.Stop() to reserve time for h.Stop() to run.
func StartAPIface(ctx context.Context, r router.Base, name string, conf *hostapd.Config, enableDNS, enableHTTP bool) (_ *APIface, retErr error) {
	ctx, st := timing.Start(ctx, "StartAPIface")
	defer st.End()

	var h APIface
	var err error

	// Validate router support.
	if rSupported, ok := r.(supportedRouter); ok {
		h.router = rSupported
	} else {
		return nil, errors.New("router type must support Hostapd and DHCP")
	}

	h.hostapd, err = h.router.StartHostapd(ctx, name, conf)
	if err != nil {
		return nil, err
	}
	defer func(ctx context.Context) {
		if retErr != nil {
			if err := h.hostapd.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to stop hostapd server while StartAPIface has failed: ", err)
			}
		}
	}(ctx)
	ctx, cancel := h.hostapd.ReserveForClose(ctx)
	defer cancel()
	h.iface = h.hostapd.Interface()

	h.subnetIdx, err = reserveSubnetIdx()
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			freeSubnetIdx(h.subnetIdx)
		}
	}()

	var dnsOpt *dhcp.DNSOption
	if enableDNS {
		dnsOpt = new(dhcp.DNSOption)
		dnsOpt.Port = dnsPort
		dnsOpt.NameServers = []string{}
		dnsOpt.ResolvedHost = ""
		dnsOpt.ResolveHostToIP = h.ServerIP()
	}

	h.dhcpd, err = h.router.StartDHCP(ctx, name, h.iface, h.subnetIP(1), h.subnetIP(128), h.ServerIP(), h.broadcastIP(), h.mask(), dnsOpt)
	if err != nil {
		return nil, err
	}

	if enableHTTP {
		h.httpServer, err = h.router.StartHTTP(ctx, name, h.iface, httpRedirectURL, httpPort, http.StatusFound)
		if err != nil {
			return nil, err
		}
	}

	return &h, nil
}

// ReserveForStop returns a shortened ctx with its cancel function.
// The shortened ctx is used for running things before h.Stop() to reserve time for it to run.
func (h *APIface) ReserveForStop(ctx context.Context) (context.Context, context.CancelFunc) {
	// We only need to call cancel of the first shorten context because the shorten context's
	// Done channel is closed when the parent context's Done channel is closed.
	// https://golang.org/pkg/context/#WithDeadline.
	var firstCancel, cancel func()
	if h.hostapd != nil {
		ctx, cancel = h.hostapd.ReserveForClose(ctx)
		if firstCancel == nil {
			firstCancel = cancel
		}
	}
	if h.dhcpd != nil {
		ctx, cancel = h.dhcpd.ReserveForClose(ctx)
		if firstCancel == nil {
			firstCancel = cancel
		}
	}
	if h.httpServer != nil {
		ctx, cancel = h.httpServer.ReserveForClose(ctx)
		if firstCancel == nil {
			firstCancel = cancel
		}
	}
	return ctx, firstCancel
}

// Stop stops the service.
func (h *APIface) Stop(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "APIface.Stop")
	defer st.End()
	if h.stopped {
		return nil
	}
	var retErr error

	// Stop DHCP
	if h.dhcpd != nil {
		if err := h.router.StopDHCP(ctx, h.dhcpd); err != nil {
			utils.CollectFirstErr(ctx, &retErr, errors.Wrap(err, "failed to stop dhcp server"))
		}
	}

	// Stop Hostapd
	if h.hostapd != nil {
		if err := h.router.StopHostapd(ctx, h.hostapd); err != nil {
			utils.CollectFirstErr(ctx, &retErr, errors.Wrap(err, "failed to stop hostapd"))
		}
	}

	// Stop HTTP server
	if h.httpServer != nil {
		if err := h.router.StopHTTP(ctx, h.httpServer); err != nil {
			retErr = errors.Wrapf(retErr, "failed to stop http server, err=%s", err.Error())
		}
	}

	freeSubnetIdx(h.subnetIdx)
	h.stopped = true
	return retErr
}

// DeauthenticateClient deauthenticates client with specified MAC address.
func (h *APIface) DeauthenticateClient(ctx context.Context, clientMAC string) error {
	return h.hostapd.DeauthClient(ctx, clientMAC)
}

// ChangeSubnetIdx restarts the dhcp server with a different subnet index.
// On failure, the APIface object will keep holding the old index, but the states of the
// dhcp server and WiFi interface are not guaranteed and a call of Stop is still needed.
func (h *APIface) ChangeSubnetIdx(ctx context.Context) (retErr error) {
	if h.dhcpd != nil {
		if err := h.router.StopDHCP(ctx, h.dhcpd); err != nil {
			return errors.Wrap(err, "failed to stop dhcp server")
		}
		h.dhcpd = nil
	}

	oldIdx := h.subnetIdx
	newIdx, err := reserveSubnetIdx()
	if err != nil {
		return errors.Wrap(err, "failed to reserve a new subnet index")
	}
	h.subnetIdx = newIdx
	defer func() {
		if retErr != nil {
			// Reset the subnet index to old value on failure.
			h.subnetIdx = oldIdx
			freeSubnetIdx(newIdx)
		} else {
			freeSubnetIdx(oldIdx)
		}
	}()
	testing.ContextLogf(ctx, "changing AP subnet index from %d to %d", oldIdx, newIdx)

	h.dhcpd, err = h.router.StartDHCP(ctx, h.name, h.iface, h.subnetIP(1), h.subnetIP(128), h.ServerIP(), h.broadcastIP(), h.mask(), nil)
	if err != nil {
		return errors.Wrap(err, "failed to start dhcp server")
	}
	return nil
}

// StartChannelSwitch initiates a channel switch in the AP.
func (h *APIface) StartChannelSwitch(ctx context.Context, count, channel int, opts ...hostapd.CSOption) error {
	return h.hostapd.StartChannelSwitch(ctx, count, channel, opts...)
}

// SendBSSTMRequest sends a BSS Transition Management Request to the specified client.
func (h *APIface) SendBSSTMRequest(ctx context.Context, clientMAC string, params hostapd.BSSTMReqParams) error {
	return h.hostapd.SendBSSTMRequest(ctx, clientMAC, params)
}

// Set sets the specified property to the specified value.
func (h *APIface) Set(ctx context.Context, prop hostapd.Property, val string) error {
	return h.hostapd.Set(ctx, prop, val)
}

// ListSTA lists the MAC addresses of connected STAs.
func (h *APIface) ListSTA(ctx context.Context) ([]string, error) {
	return h.hostapd.ListSTA(ctx)
}

// STAInfo queries information of the connected STA.
func (h *APIface) STAInfo(ctx context.Context, staMAC string) (*hostapd.STAInfo, error) {
	return h.hostapd.STAInfo(ctx, staMAC)
}

// SendBeaconRequest sends a Beacon Request to the specified client.
func (h *APIface) SendBeaconRequest(ctx context.Context, clientMAC string, params hostapd.BeaconReqParams) error {
	return h.hostapd.SendBeaconRequest(ctx, clientMAC, params)
}

// Router returns the current router used by the AP.
func (h *APIface) Router() router.Base {
	return h.router
}
