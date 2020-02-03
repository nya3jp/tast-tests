// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"fmt"
	"net"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	remote_iw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

// InterfaceHandle is the handle object of an instance of hostapd service managed by Router.
// It is comprised of a hostapd and a dhcpd. The DHCP server is assigned with the subnet
// 192.168.$subnetIdx.0/24.
type InterfaceHandle struct {
	host      *host.SSH
	name      string
	iface     string
	workDir   string
	subnetIdx byte
	config    *hostapd.Config

	hostapd *hostapd.Server
	dhcpd   *dhcp.Server
}

// Config returns the config of hostapd.
func (h *InterfaceHandle) Config() hostapd.Config {
	return *h.config
}

// subnetIP returns 192.168.$subnetIdx.$suffix IP.
func (h *InterfaceHandle) subnetIP(suffix byte) net.IP {
	return net.IPv4(192, 168, h.subnetIdx, suffix)
}

// mask returns the mask of the working subnet.
func (h *InterfaceHandle) mask() net.IPMask {
	return net.IPv4Mask(255, 255, 255, 0)
}

// broadcastIP returns the broadcast IP of working subnet.
func (h *InterfaceHandle) broadcastIP() net.IP {
	return h.subnetIP(255)
}

// ServerIP returns the IP of router in the subnet of WiFi.
func (h *InterfaceHandle) ServerIP() net.IP {
	return h.subnetIP(254)
}

// start the service. Make this private as one should start this from Router.
func (h *InterfaceHandle) start(ctx context.Context) (retErr error) {
	defer func() {
		if retErr == nil {
			return
		}
		if err := h.stop(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to stop HostAPHandle, err=%s", err.Error())
		}
	}()

	if err := h.setupIface(ctx); err != nil {
		return errors.Wrap(err, "failed to setup interface")
	}

	hs := hostapd.NewServer(h.host, h.name, h.iface, h.workDir, h.config)
	if err := hs.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	h.hostapd = hs

	ds := dhcp.NewServer(h.host, h.name, h.iface, h.workDir, h.subnetIP(1), h.subnetIP(128))
	if err := ds.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start dhcp server")
	}
	h.dhcpd = ds

	return nil
}

// stop the service. Make this private as one should stop it from Router.
func (h *InterfaceHandle) stop(ctx context.Context) error {
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
	if err := h.teardownIface(ctx); err != nil {
		retErr = errors.Wrapf(retErr, "teardownIface error=%s", err.Error())
	}
	return retErr
}

// setupIface sets up the interface which we're providing services on.
func (h *InterfaceHandle) setupIface(ctx context.Context) error {
	var retErr error
	if err := remote_iw.NewRunner(h.host).SetTxPowerAuto(ctx, h.iface); err != nil {
		retErr = errors.Wrapf(retErr, "failed to setup txpower to auto, err=%s", err)
	}
	if err := h.configureIP(ctx); err != nil {
		retErr = errors.Wrapf(retErr, "failed to configureIP, err=%s", err)
	}
	return retErr
}

// teardownIface tears down the interface which we provided services on.
func (h *InterfaceHandle) teardownIface(ctx context.Context) error {
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
func (h *InterfaceHandle) flushIP(ctx context.Context) error {
	if err := h.host.Command("ip", "addr", "flush", h.iface).Run(ctx); err != nil {
		return errors.Wrap(err, "failed to flush address")
	}
	return nil
}

// configureIP configures server IP and broadcast IP on h.iface.
func (h *InterfaceHandle) configureIP(ctx context.Context) error {
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
