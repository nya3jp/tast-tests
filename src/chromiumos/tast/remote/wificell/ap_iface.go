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
	remote_ip "chromiumos/tast/remote/network/ip"
	remote_iw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// APIface is the handle object of an instance of hostapd service managed by Router.
// It is comprised of a hostapd and a dhcpd. The DHCP server is assigned with the subnet
// 192.168.$subnetIdx.0/24.
type APIface struct {
	host      *ssh.Conn
	name      string
	iface     string
	workDir   string
	subnetIdx byte
	config    *hostapd.Config

	hostapd *hostapd.Server
	dhcpd   *dhcp.Server

	stopped bool // true if stop() is called. Used to avoid stop() being called twice.
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

// start starts the service. Make this private as one should start this from Router.
// After start(), the caller should call h.stop() at the end, and use the shortened ctx
// (provided b h.reserveForStop()) before h.stop() to reserve time for h.stop() to run.
func (h *APIface) start(fullCtx context.Context) (retErr error) {
	defer func() {
		if retErr != nil {
			if err := h.stop(fullCtx); err != nil {
				testing.ContextLogf(fullCtx, "Failed to stop HostAPHandle, err=%s", err.Error())
			}
		}
	}()
	h.stopped = false

	// Reserve for h.tearDownIface() running in h.stop().
	// Calling h.reserveForStop() here reserves insufficient time because h.hostapd and h.dhcpd
	// are not set yet.
	ctx, cancel := ctxutil.Shorten(fullCtx, time.Second)
	defer cancel()

	if err := h.configureIface(ctx); err != nil {
		return errors.Wrap(err, "failed to setup interface")
	}

	hs, err := hostapd.StartServer(ctx, h.host, h.name, h.iface, h.workDir, h.config)
	if err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	h.hostapd = hs
	// We only need to call cancel of the first shorten context.
	ctx, _ = h.hostapd.ReserveForClose(ctx)

	ds, err := dhcp.StartServer(ctx, h.host, h.name, h.iface, h.workDir, h.subnetIP(1), h.subnetIP(128))
	if err != nil {
		return errors.Wrap(err, "failed to start dhcp server")
	}
	h.dhcpd = ds

	return nil
}

// reserveForStop returns a shortened ctx with its cancel function.
// The shortened ctx is used for running things before h.stop() to reserve time for it to run.
func (h *APIface) reserveForStop(ctx context.Context) (context.Context, context.CancelFunc) {
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

// stop stops the service. Make this private as one should stop it from Router.
func (h *APIface) stop(ctx context.Context) error {
	if h.stopped {
		return nil
	}
	var retErr error
	if h.dhcpd != nil {
		if err := h.dhcpd.Close(ctx); err != nil {
			retErr = errors.Wrapf(retErr, "failed to stop dhcp server, err=%s", err.Error())
		}
	}
	if h.hostapd != nil {
		if err := h.hostapd.Close(ctx); err != nil {
			retErr = errors.Wrapf(retErr, "failed to stop hostapd, err=%s", err.Error())
		}
	}
	if err := h.tearDownIface(ctx); err != nil {
		retErr = errors.Wrapf(retErr, "teardownIface error=%s", err.Error())
	}
	h.stopped = true
	return retErr
}

// configureIface configures the interface which we're providing services on.
func (h *APIface) configureIface(ctx context.Context) error {
	var retErr error
	if err := remote_iw.NewRemoteRunner(h.host).SetTxPowerAuto(ctx, h.iface); err != nil {
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
	ipr := remote_ip.NewRemoteRunner(h.host)
	if err := ipr.FlushIP(ctx, h.iface); err != nil {
		collectFirstErr(ctx, &firstErr, err)
	}
	if err := ipr.SetLinkDown(ctx, h.iface); err != nil {
		collectFirstErr(ctx, &firstErr, err)
	}
	return firstErr
}

// configureIP configures server IP and broadcast IP on h.iface.
func (h *APIface) configureIP(ctx context.Context) error {
	ipr := remote_ip.NewRemoteRunner(h.host)
	if err := ipr.FlushIP(ctx, h.iface); err != nil {
		return err
	}
	maskLen, _ := h.mask().Size()
	return ipr.AddIP(ctx, h.iface, h.ServerIP(), maskLen, ip.AddIPBroadcast(h.broadcastIP()))
}

// changeSubnetIdx configures the ip to the new index and restart the dhcp server.
// On failure, the APIface object will keep holding the old index, but the states of the
// dhcp server and WiFi interface are not guaranteed and a call of stop is still needed.
func (h *APIface) changeSubnetIdx(ctx context.Context, newIdx byte) (retErr error) {
	if h.dhcpd != nil {
		if err := h.dhcpd.Close(ctx); err != nil {
			return errors.Wrap(err, "failed to stop dhcp server")
		}
		h.dhcpd = nil
	}

	// Reset the subnet index to old value on failure.
	oldIdx := h.subnetIdx
	defer func() {
		if retErr != nil {
			h.subnetIdx = oldIdx
		}
	}()

	h.subnetIdx = newIdx
	if err := h.configureIP(ctx); err != nil {
		return errors.Wrap(err, "failed to configure ip")
	}

	ds, err := dhcp.StartServer(ctx, h.host, h.name, h.iface, h.workDir, h.subnetIP(1), h.subnetIP(128))
	if err != nil {
		return errors.Wrap(err, "failed to start dhcp server")
	}
	h.dhcpd = ds
	return nil
}
