// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"fmt"
	"net"

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

// start the service. Make this private as one should start this from Router.
func (h *APIface) start(ctx context.Context) (retErr error) {
	defer func() {
		if retErr == nil {
			return
		}
		if err := h.stop(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to stop HostAPHandle, err=%s", err.Error())
		}
	}()

	if err := h.configureIface(ctx); err != nil {
		return errors.Wrap(err, "failed to setup interface")
	}

	hs, err := hostapd.StartServer(ctx, h.host, h.name, h.iface, h.workDir, h.config)
	if err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	h.hostapd = hs

	ds, err := dhcp.StartServer(ctx, h.host, h.name, h.iface, h.workDir, h.subnetIP(1), h.subnetIP(128))
	if err != nil {
		return errors.Wrap(err, "failed to start dhcp server")
	}
	h.dhcpd = ds

	return nil
}

// stop the service. Make this private as one should stop it from Router.
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
func (h *APIface) configureIface(ctx context.Context) error {
	var retErr error
	if err := remote_iw.NewRunner(h.host).SetTxPowerAuto(ctx, h.iface); err != nil {
		retErr = errors.Wrapf(retErr, "failed to set txpower to auto, err=%s", err)
	}
	if err := h.configureIP(ctx); err != nil {
		retErr = errors.Wrapf(retErr, "failed to configureIP, err=%s", err)
	}
	return retErr
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
