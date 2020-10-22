// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"net"

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

	h.router = r

	h.hostapd, err = r.StartHostapd(ctx, name, conf)
	if err != nil {
		return nil, err
	}
	defer func(ctx context.Context) {
		if retErr != nil {
			if err := h.hostapd.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to stop hostapd server while NewAPIface has failed: ", err)
			}
		}
	}(ctx)
	ctx, cancel := h.hostapd.ReserveForClose(ctx)
	defer cancel()
	h.iface = h.hostapd.Interface()
	// hostapd.Config() makes copy, so calling hostapd.Config() in each APIface.Config() would be heavy. Make copy only once here.
	config := h.hostapd.Config()
	h.config = &config

	h.subnetIdx, err = reserveSubnetIdx()
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			freeSubnetIdx(h.subnetIdx)
		}
	}()

	h.dhcpd, err = r.StartDHCP(ctx, name, h.iface, h.subnetIP(1), h.subnetIP(128), h.ServerIP(), h.broadcastIP(), h.mask())
	if err != nil {
		return nil, err
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
	return ctx, firstCancel
}

// Stop stops the service.
func (h *APIface) Stop(ctx context.Context) error {
	if h.stopped {
		return nil
	}
	var retErr error
	if h.dhcpd != nil {
		if err := h.router.StopDHCP(ctx, h.dhcpd, h.iface); err != nil {
			retErr = errors.Wrapf(retErr, "failed to stop dhcp server, err=%s", err.Error())
		}
	}
	if h.hostapd != nil {
		if err := h.router.StopHostapd(ctx, h.hostapd, h.iface); err != nil {
			retErr = errors.Wrapf(retErr, "failed to stop hostapd, err=%s", err.Error())
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
		if err := h.router.StopDHCP(ctx, h.dhcpd, h.iface); err != nil {
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

	h.dhcpd, err = h.router.StartDHCP(ctx, h.name, h.iface, h.subnetIP(1), h.subnetIP(128), h.ServerIP(), h.broadcastIP(), h.mask())
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

	if err := h.router.StopHostapd(ctx, h.hostapd, h.iface); err != nil {
		return errors.Wrap(err, "failed to stop hostapd")
	}
	h.hostapd = nil

	h.config.SSID = ssid
	var err error
	h.hostapd, err = h.router.StartHostapd(ctx, h.name, h.config)
	if err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	h.iface = h.hostapd.Interface()
	config := h.hostapd.Config()
	h.config = &config
	return nil
}
