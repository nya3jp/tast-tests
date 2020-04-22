// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"fmt"
	"net"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
}

// Config returns the config of hostapd.
func (h *APIface) Config() hostapd.Config {
	return *h.config
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

// start starts the service and returns shortCtx.
// The caller should call h.stop() to perform clean-up. And the shortened context is used to
// reserve time for h.stop() to run.
// Make this private as one should start this from Router.
func (h *APIface) start(fullCtx context.Context) (
	shortCtx context.Context, shortCtxCancel context.CancelFunc, retErr error) {
	// Note that it shortens ctx three times. We only need to call cancel of the first shortened context
	// because the shorten context's Done channel is closed when the parent context's Done channel is closed.
	sCtx, sCtxCancel, err := h.configureIface(fullCtx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to setup interface")
	}
	defer func() {
		if retErr != nil {
			sCtxCancel()
			if err := h.stop(fullCtx); err != nil {
				testing.ContextLogf(fullCtx, "Failed to stop HostAPHandle, err=%s", err.Error())
			}
		}
	}()

	hs, sCtx1, _, err := hostapd.StartServer(sCtx, h.host, h.name, h.iface, h.workDir, h.config)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start hostapd")
	}
	h.hostapd = hs

	ds, sCtx2, _, err := dhcp.StartServer(sCtx1, h.host, h.name, h.iface, h.workDir, h.subnetIP(1), h.subnetIP(128))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start dhcp server")
	}
	h.dhcpd = ds

	return sCtx2, sCtxCancel, nil
}

// stop stops the service. Make this private as one should stop it from Router.
func (h *APIface) stop(ctx context.Context) error {
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
	return retErr
}

// configureIface configures the interface which we're providing services on.
// It also returns shortCtx to reserve time for h.tearDownIface().
func (h *APIface) configureIface(fullCtx context.Context) (
	shortCtx context.Context, shortCtxCancel context.CancelFunc, retErr error) {
	// Shorten ctx to reserve time for d.Close() to run.
	ctx, cancel := ctxutil.Shorten(fullCtx, time.Second)
	defer func() {
		if retErr != nil {
			cancel()
			h.tearDownIface(fullCtx)
		}
	}()

	if err := remote_iw.NewRunner(h.host).SetTxPowerAuto(ctx, h.iface); err != nil {
		retErr = errors.Wrapf(retErr, "failed to set txpower to auto, err=%s", err)
	}
	if err := h.configureIP(ctx); err != nil {
		retErr = errors.Wrapf(retErr, "failed to configureIP, err=%s", err)
	}
	if retErr != nil {
		return nil, nil, retErr
	}
	return ctx, cancel, nil
}

// tearDownIface tears down the interface which we provided services on.
func (h *APIface) tearDownIface(ctx context.Context) error {
	var retErr error
	if err := h.flushIP(ctx); err != nil {
		retErr = err
	}
	if err := h.host.Command("ip", "link", "set", h.iface, "down").Run(ctx); err != nil {
		err = errors.Wrapf(retErr, "failed to set %s down, err=%s", h.iface, err.Error())
	}
	return nil
}

// flushIP flushes the IP setting on h.iface.
func (h *APIface) flushIP(ctx context.Context) error {
	if err := h.host.Command("ip", "addr", "flush", h.iface).Run(ctx); err != nil {
		return errors.Wrap(err, "failed to flush address")
	}
	return nil
}

// configureIP configures server IP and broadcast IP on h.iface.
func (h *APIface) configureIP(ctx context.Context) error {
	if err := h.flushIP(ctx); err != nil {
		return err
	}
	maskLen, _ := h.mask().Size()
	cmd := h.host.Command("ip", "addr", "add", fmt.Sprintf("%s/%d", h.ServerIP().String(), maskLen),
		"broadcast", h.broadcastIP().String(), "dev", h.iface)
	if err := cmd.Run(ctx); err != nil {
		return errors.Wrap(err, "failed to add address")
	}
	return nil
}
