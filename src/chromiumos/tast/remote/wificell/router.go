// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	remote_iw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

const workingDir = "/tmp/tast-test/"

// Router is the object to control an AP router.
type Router struct {
	host       *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	ifaces     []*iw.NetDev
	busySubnet map[byte]struct{}
	busyIfaces map[string]struct{}
	nextID     int
}

// NewRouter connects to the router by SSH and creates a Router object.
func NewRouter(ctx context.Context, host *host.SSH) (*Router, error) {
	r := &Router{
		host:       host,
		busySubnet: make(map[byte]struct{}),
		busyIfaces: make(map[string]struct{}),
	}
	if err := r.initialize(ctx); err != nil {
		r.Close(ctx)
		return nil, err
	}

	return r, nil
}

// removeWifiIface with name iface.
func (r *Router) removeWifiIface(ctx context.Context, iface string) error {
	testing.ContextLogf(ctx, "Deleting wdev %s on router", iface)
	if out, err := r.host.Command("iw", "dev", iface, "del").Output(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete wdev %s: %s", iface, string(out))
	}
	return nil
}

// createWifiIface on phy with interface name=iface and type=ifaceType.
func (r *Router) createWifiIface(ctx context.Context, phy, iface, ifaceType string) error {
	testing.ContextLogf(ctx, "Creating wdev %s on wiphy %s", iface, phy)
	cmd := r.host.Command("iw", "phy", phy, "interface", "add", iface, "type", ifaceType)
	if out, err := cmd.Output(ctx); err != nil {
		return errors.Wrapf(err, "failed to create wdev %s on wiphy %s: %s", iface, phy, string(out))
	}
	return nil
}

// removeWifiIfaces listed by "iw dev".
func (r *Router) removeWifiIfaces(ctx context.Context) error {
	iwr := remote_iw.NewRunner(r.host)
	wdevs, err := iwr.ListInterfaces(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list interfaces")
	}
	for _, w := range wdevs {
		if err := r.removeWifiIface(ctx, w.IfName); err != nil {
			return err
		}
	}
	return nil
}

// setupWifiIfaces enables antenna and creates interfaces on phys.
func (r *Router) setupWifiIfaces(ctx context.Context) error {
	if err := r.removeWifiIfaces(ctx); err != nil {
		return err
	}
	iwr := remote_iw.NewRunner(r.host)
	wiphys, err := iwr.ListPhys(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list phys")
	}
	for i, p := range wiphys {
		if p.SupportSetAntennaMask() {
			if err := iwr.SetAntennaBitmap(ctx, p.Name, p.TxAntenna, p.RxAntenna); err != nil {
				return errors.Wrapf(err, "failed to set bitmap for %s", p.Name)
			}
		}
		iface := fmt.Sprintf("managed%d", i)
		r.createWifiIface(ctx, p.Name, iface, "managed")
	}
	return nil
}

// initialize prepares initial test AP state (e.g., initializing wiphy/wdev).
func (r *Router) initialize(ctx context.Context) error {
	// Setup working dir.
	if err := r.host.Command("rm", "-rf", r.workDir()).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to remove workdir %q", r.workDir())
	}
	if err := r.host.Command("mkdir", "-p", r.workDir()).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to create workdir %q", r.workDir())
	}

	// Kill remaining hostapd/dnsmasq.
	hostapd.Killall(ctx, r.host)
	dhcp.Killall(ctx, r.host)
	// Stop upstart job wpasupplicant if available. (ignore the error as it might be stopped already)
	r.host.Command("stop", "wpasupplicant").Run(ctx)

	// TODO(crbug.com/839164): Verify if we still need to truncate uma-events.

	iwr := remote_iw.NewRunner(r.host)
	// Set default regulatory domain to US.
	if err := iwr.SetRegulatoryDomain(ctx, "US"); err != nil {
		return errors.Wrap(err, "failed to set regulatory domain to US")
	}

	// Set up phys and interfaces.
	if err := r.setupWifiIfaces(ctx); err != nil {
		return err
	}

	// TODO(crbug.com/774808): configure hw_random.

	// After setting up WiFi interfaces, obtain a copy of interfaces in r.ifaces.
	ifaces, err := iwr.ListInterfaces(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get list of interfaces")
	}
	r.ifaces = ifaces
	return nil
}

// Close cleans the resource used by Router.
func (r *Router) Close(ctx context.Context) error {
	var err error
	if err2 := r.removeWifiIfaces(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to remove interfaces, err=%s", err2.Error())
	}
	if err2 := r.collectLogs(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to collect logs, err=%s", err2.Error())
	}
	if err2 := r.host.Command("rm", "-rf", r.workDir()).Run(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to remove working dir, err=%s", err2.Error())
	}
	return err
}

// SelectInterface finds an available interface suitable for the given hostap config.
func (r *Router) SelectInterface(ctx context.Context, conf *hostapd.Config) (string, error) {
	freq, err := hostapd.ChannelToFrequency(conf.Channel)
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

// getUniqueID returns an unique ID string on this router. Useful for giving names to daemons/services.
func (r *Router) getUniqueID() string {
	id := strconv.Itoa(r.nextID)
	r.nextID++
	return id
}

// StartHostAP starts a hostap service which includes hostapd and dhcpd. The handle object for the
// service is returned.
func (r *Router) StartHostAP(ctx context.Context, conf *hostapd.Config) (*HostAPHandle, error) {
	// Reserve required resources.
	name := r.getUniqueID()
	iface, err := r.SelectInterface(ctx, conf)
	if err != nil {
		return nil, err
	}
	r.setIfaceBusy(iface, true)

	idx, err := r.getSubnetIdx()
	if err != nil {
		r.setIfaceBusy(iface, false)
		return nil, err
	}

	h := &HostAPHandle{
		host:      r.host,
		name:      name,
		iface:     iface,
		workDir:   r.workDir(),
		subnetIdx: idx,
		config:    conf,
	}
	if err := h.start(ctx); err != nil {
		// Release resources.
		r.freeSubnetIdx(idx)
		r.setIfaceBusy(iface, false)
		return nil, err
	}
	return h, nil
}

// StopHostAP stops a hostap service.
func (r *Router) StopHostAP(ctx context.Context, h *HostAPHandle) error {
	r.freeSubnetIdx(h.subnetIdx)
	r.setIfaceBusy(h.iface, false)
	return h.stop(ctx)
}

func (r *Router) workDir() string {
	return workingDir
}

// Utilities for resource control.

// getSubnetIdx finds a free subnet index and reserves it.
func (r *Router) getSubnetIdx() (byte, error) {
	for i := byte(0); i <= 255; i++ {
		if _, ok := r.busySubnet[i]; ok {
			continue
		}
		r.busySubnet[i] = struct{}{}
		return i, nil
	}
	return 0, errors.New("subnet index exhausted")
}

// freeSubnetIdx marks the subnet index as unused.
func (r *Router) freeSubnetIdx(i byte) {
	delete(r.busySubnet, i)
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

func (r *Router) collectLogs(ctx context.Context) error {
	collect := map[string]string{
		"/var/log/messages": "debug/router_host_messages",
	}
	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("OutDir not supported")
	}
	for s, d := range collect {
		dst := path.Join(outdir, d)
		basedir := path.Dir(dst)
		if err := os.MkdirAll(basedir, 0755); err != nil {
			return errors.Wrapf(err, "failed to mkdir %s", basedir)
		}
		if err := r.host.GetFile(ctx, s, dst); err != nil {
			return errors.Wrapf(err, "failed to download %s to %s", s, dst)
		}
	}
	return nil
}
