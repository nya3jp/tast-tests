// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"net"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	remote_iw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wifi/dhcp"
	"chromiumos/tast/remote/wifi/hostap"
	"chromiumos/tast/remote/wifi/utils"
	"chromiumos/tast/testing"
)

// Ported from Brian's draft crrev.com/c/1733740

const workingDir = "/tmp/tast-test/"

// Router is the object to control an AP router.
type Router struct {
	host        *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	ifaces      []*iw.NetDev
	subnetInUse [256]bool
	busyIfaces  map[string]struct{}
}

// HostAPHandle is the handle object of the HostAP service managed under Router object.
type HostAPHandle struct {
	hostapd *hostap.Server
	dhcpd   *dhcp.Server
}

// Config returns the config of hostapd.
func (h *HostAPHandle) Config() hostap.Config {
	return h.hostapd.Config()
}

// ServerIP returns the IP of router in the subnet of WiFi.
func (h *HostAPHandle) ServerIP() net.IP {
	return h.dhcpd.ServerIP()
}

// NewRouter connects to the router by SSH and creates a Router object.
func NewRouter(ctx context.Context, hst *host.SSH) (*Router, error) {
	r := &Router{
		host:       hst,
		busyIfaces: make(map[string]struct{}),
	}
	if err := r.initialize(ctx); err != nil {
		r.Close(ctx)
		return nil, err
	}

	return r, nil
}

func (r *Router) removeWifiIfaces(ctx context.Context) error {
	iwr := remote_iw.NewRunner(r.host)
	wdevs, err := iwr.ListInterfaces(ctx)
	if err != nil {
		return err
	}
	for _, w := range wdevs {
		testing.ContextLogf(ctx, "Deleting wdev %s on router", w.IfName)
		if out, err := r.host.Command("iw", "dev", w.IfName, "del").Output(ctx); err != nil {
			return errors.Wrapf(err, "failed to delete wdev %s: %s", w.IfName, string(out))
		}
	}
	return nil
}

// initialize prepares initial test AP state (e.g., initializing wiphy/wdev).
func (r *Router) initialize(ctx context.Context) error {
	// Setup working dir.
	if err := r.host.Command("rm", "-rf", r.workDir()).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to cleanup %q", r.workDir())
	}
	if err := r.host.Command("mkdir", "-p", r.workDir()).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to create workdir %q", r.workDir())
	}

	// Create interfaces on phys.
	if err := r.removeWifiIfaces(ctx); err != nil {
		return err
	}
	iwr := remote_iw.NewRunner(r.host)
	wiphys, err := iwr.ListPhys(ctx)
	if err != nil {
		return err
	}
	for i, p := range wiphys {
		iface := fmt.Sprintf("managed%d", i)
		testing.ContextLogf(ctx, "Creating wdev %s on wiphy %s", iface, p.Name)
		cmd := r.host.Command("iw", "phy", p.Name, "interface", "add", iface, "type", "managed")
		if out, err := cmd.Output(ctx); err != nil {
			return errors.Wrapf(err, "failed to create wdev %s on wiphy %s: %s", iface, p.Name, string(out))
		}
	}

	// Get interface list again after creation.
	ifaces, err := iwr.ListInterfaces(ctx)
	if err != nil {
		return err
	}
	// Keep a cache of the interface list on r.
	r.ifaces = ifaces
	return err
}

// Close disconnects the SSH to router.
func (r *Router) Close(ctx context.Context) error {
	var err error
	if err2 := r.removeWifiIfaces(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to remove interfaces, err=%s", err2.Error())
	}
	if err2 := r.host.Command("rm", "-rf", r.workDir()).Run(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to remove working dir, err=%s", err2.Error())
	}
	return err
}

// SelectInterface finds an available interface suitable for the given hostap config.
func (r *Router) SelectInterface(ctx context.Context, conf *hostap.Config) (string, error) {
	freq, err := utils.ChannelToFrequency(conf.Channel)
	if err != nil {
		return "", errors.Errorf("channel %d not available", conf.Channel)
	}
	iwr := remote_iw.NewRunner(r.host)
	for _, iface := range r.ifaces {
		if r.isIfaceBusy(iface.IfName) {
			continue
		}
		phy, err := iwr.GetPhyByID(ctx, iface.PhyNum)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get phy of interface %s, id=%d", iface.IfName, iface.PhyNum)
		}

		supported := false
		// Check channel support.
		for _, b := range phy.Bands {
			if _, ok := b.FrequencyFlags[freq]; ok {
				supported = true
				break
			}
		}
		if supported {
			return iface.IfName, nil
		}
	}
	return "", errors.New("cannot find supported interface")
}

// StartHostAP starts a hostap service which includes hostapd and dhcpd. The handle object for the
// service is returned.
func (r *Router) StartHostAP(ctx context.Context, conf *hostap.Config) (ret *HostAPHandle, err error) {
	hostapd, err := r.startHostapd(ctx, conf)
	if err != nil {
		return nil, err
	}
	// Cleanup if something fails later.
	defer func() {
		if err != nil {
			r.stopHostapd(ctx, hostapd)
		}
	}()

	dhcpd, err := r.startDHCPd(ctx, hostapd.Interface())
	if err != nil {
		return nil, err
	}
	// Cleanup if something fails later.
	defer func() {
		if err != nil {
			r.stopDHCPd(ctx, dhcpd)
		}
	}()

	return &HostAPHandle{
		hostapd: hostapd,
		dhcpd:   dhcpd,
	}, nil
}

// StopHostAP stops a hostap service.
func (r *Router) StopHostAP(ctx context.Context, h *HostAPHandle) error {
	var err error
	if err2 := r.stopHostapd(ctx, h.hostapd); err2 != nil {
		err = errors.Wrap(err2, "failed to stop hostap")
	}
	if err2 := r.stopDHCPd(ctx, h.dhcpd); err2 != nil {
		err2 = errors.Wrap(err2, "failed to stop dhcp")
		err = errors.Wrap(err, err2.Error())
	}
	return err
}

func (r *Router) workDir() string {
	return workingDir
}

// Utilities for resource control.

func (r *Router) startHostapd(ctx context.Context, conf *hostap.Config) (*hostap.Server, error) {
	iface, err := r.SelectInterface(ctx, conf)
	if err != nil {
		return nil, err
	}
	r.setIfaceBusy(iface, true)
	return hostap.NewServer(ctx, r.host, iface, r.workDir(), conf)
}

func (r *Router) stopHostapd(ctx context.Context, hostapd *hostap.Server) error {
	r.setIfaceBusy(hostapd.Interface(), false)
	return hostapd.Stop(ctx)
}

func (r *Router) startDHCPd(ctx context.Context, iface string) (*dhcp.Server, error) {
	idx, err := r.getSubnetIdx()
	if err != nil {
		return nil, err
	}
	dhcpConf := dhcp.NewConfig(idx)
	return dhcp.NewServer(ctx, r.host, iface, r.workDir(), dhcpConf)
}

func (r *Router) stopDHCPd(ctx context.Context, dhcpd *dhcp.Server) error {
	r.freeSubnetIdx(dhcpd.Config().SubnetIdx)
	return dhcpd.Stop(ctx)
}

func (r *Router) getSubnetIdx() (byte, error) {
	for i, v := range r.subnetInUse {
		if v {
			continue
		}
		r.subnetInUse[i] = true
		return byte(i), nil
	}
	return 0, errors.New("ip index exhausted")
}

func (r *Router) freeSubnetIdx(i byte) {
	if int(i) >= len(r.subnetInUse) {
		return
	}
	r.subnetInUse[i] = false
}

func (r *Router) setIfaceBusy(iface string, busy bool) {
	if busy {
		r.busyIfaces[iface] = struct{}{}
	} else {
		delete(r.busyIfaces, iface)
	}
}

func (r *Router) isIfaceBusy(iface string) bool {
	_, ok := r.busyIfaces[iface]
	return ok
}
