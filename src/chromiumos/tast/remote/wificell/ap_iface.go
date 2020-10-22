// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"net"
	"time"

	"chromiumos/tast/common/network/ip"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
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

// APIface is the handle object of an instance of hostapd service managed by Router.
// It is comprised of a hostapd and a dhcpd. The DHCP server is assigned with the subnet
// 192.168.$subnetIdx.0/24.
type APIface struct {
	router    *Router
	name      string
	iface     string
	subnetIdx byte
	config    *hostapd.Config

	hostapd *hostapd.Server
	dhcpd   *dhcp.Server

	stopped bool // true if Stop() is called. Used to avoid Stop() being called twice.
}

// Config returns the config of hostapd.
// NOTE: Caller should not modify the returned object.
func (h *APIface) Config() *hostapd.Config {
	return h.config
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

// NewAPIface starts the service.
// After started, the caller should call h.Stop() at the end, and use the shortened ctx
// (provided by h.ReserveForStop()) before h.Stop() to reserve time for h.Stop() to run.
func NewAPIface(ctx context.Context, r *Router, name string, conf *hostapd.Config) (_ *APIface, retErr error) {
	var h APIface
	var err error

	defer func() {
		if retErr != nil {
			if err := h.Stop(ctx); err != nil {
				testing.ContextLogf(ctx, "Failed to stop HostAPHandle, err=%s", err.Error())
			}
		}
	}()

	h.router = r
	h.config = conf

	h.hostapd, h.iface, err = r.StartHostapd(ctx, name, conf)
	if err != nil {
		return nil, err
	}
	ctx, cancel := h.hostapd.ReserveForClose(ctx)
	defer cancel()

	h.subnetIdx, err = reserveSubnetIdx()
	if err != nil {
		return nil, err
	}
	// FIXME: Should we move all the IP configuring methods to Router?
	if err = h.configureIface(ctx); err != nil {
		return nil, err
	}
	// Reserve for tearDownIface.
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	// FIXME: Should we configure IP in StartDHCP instead?
	h.dhcpd, err = r.StartDHCP(ctx, name, h.iface, h.subnetIP(1), h.subnetIP(128))
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// ReserveForStop returns a shortened ctx with its cancel function.
// The shortened ctx is used for running things before h.Stop() to reserve time for it to run.
func (h *APIface) ReserveForStop(ctx context.Context) (context.Context, context.CancelFunc) {
	// Reserve for h.tearDownIface()
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	if h.hostapd != nil {
		// We only need to call cancel of the first shorten context because the shorten context's
		// Done channel is closed when the parent context's Done channel is closed.
		// https://golang.org/pkg/context/#WithDeadline.
		ctx, _ = h.hostapd.ReserveForClose(ctx)
	}
	if h.dhcpd != nil {
		ctx, _ = h.dhcpd.ReserveForClose(ctx)
	}
	return ctx, cancel
}

// Stop stops the service.
func (h *APIface) Stop(ctx context.Context) error {
	if h.stopped {
		return nil
	}
	var retErr error
	if h.dhcpd != nil {
		if err := h.router.StopDHCP(ctx, h.dhcpd); err != nil {
			retErr = errors.Wrapf(retErr, "failed to stop dhcp server, err=%s", err.Error())
		}
	}
	if h.hostapd != nil {
		if err := h.router.StopHostapd(ctx, h.iface, h.hostapd); err != nil {
			retErr = errors.Wrapf(retErr, "failed to stop hostapd, err=%s", err.Error())
		}
	}
	if h.iface != "" {
		if err := h.tearDownIface(ctx); err != nil {
			retErr = errors.Wrapf(retErr, "teardownIface error=%s", err.Error())
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

// configureIface configures the interface which we're providing services on.
func (h *APIface) configureIface(ctx context.Context) error {
	var retErr error
	if err := h.router.iwr.SetTxPowerAuto(ctx, h.iface); err != nil {
		retErr = errors.Wrapf(retErr, "failed to set txpower to auto, err=%s", err)
	}
	if err := h.configureIP(ctx); err != nil {
		retErr = errors.Wrapf(retErr, "failed to configureIP, err=%s", err)
	}
	return retErr
}

// tearDownIface tears down the interface which we provided services on.
func (h *APIface) tearDownIface(ctx context.Context) error {
	var firstErr error
	if err := h.router.ipr.FlushIP(ctx, h.iface); err != nil {
		collectFirstErr(ctx, &firstErr, err)
	}
	// FIXME: Since the iface is enabled by hostapd, should we move this to StopHostapd?
	if err := h.router.ipr.SetLinkDown(ctx, h.iface); err != nil {
		collectFirstErr(ctx, &firstErr, err)
	}
	return firstErr
}

// configureIP configures server IP and broadcast IP on h.iface.
func (h *APIface) configureIP(ctx context.Context) error {
	if err := h.router.ipr.FlushIP(ctx, h.iface); err != nil {
		return err
	}
	maskLen, _ := h.mask().Size()
	return h.router.ipr.AddIP(ctx, h.iface, h.ServerIP(), maskLen, ip.AddIPBroadcast(h.broadcastIP()))
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

	if err := h.configureIP(ctx); err != nil {
		return errors.Wrap(err, "failed to configure ip")
	}

	h.dhcpd, err = h.router.StartDHCP(ctx, h.name, h.iface, h.subnetIP(1), h.subnetIP(128))
	if err != nil {
		return errors.Wrap(err, "failed to start dhcp server")
	}
	return nil
}

// ChangeSSID changes the SSID without changing other settings, such as IP address and interface.
// On failure, a call of stop is still needed to deconfigure the dhcp server and the WiFi interface.
func (h *APIface) ChangeSSID(ctx context.Context, ssid string) error {
	if h.stopped || h.hostapd == nil {
		return errors.New("hostapd is not running")
	}

	if err := h.router.StopHostapd(ctx, h.iface, h.hostapd); err != nil {
		return errors.Wrap(err, "failed to stop hostapd")
	}
	h.hostapd = nil

	// hostapd will attempt to set the interface up and would fail if it is already up.
	if err := h.router.ipr.SetLinkDown(ctx, h.iface); err != nil {
		return err
	}

	h.config.SSID = ssid
	var err error
	h.hostapd, h.iface, err = h.router.StartHostapd(ctx, h.name, h.config)
	if err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	return nil
}
