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
	"strings"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	remote_iw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

const workingDir = "/tmp/tast-test/"

type ifaceType string

const (
	ifaceTypeManaged ifaceType = "managed"
)

// netDev contains the information of an interface.
type netDev struct {
	phyName   string
	ifaceType ifaceType
}

// Router is the object to control an AP router.
type Router struct {
	host        *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	ifaces      []*iw.NetDev
	board       string
	busySubnet  map[byte]struct{}
	phys        []*iw.Phy
	busyPhy     map[string]int     // map from phy name to busy interface count.
	availIfaces map[string]*netDev // map from interface name to netDev.
	busyIfaces  map[string]*netDev // map from interface name to netDev.
	handleID    int
	ifaceID     int
}

// NewRouter connects to the router by SSH and creates a Router object.
func NewRouter(ctx context.Context, host *host.SSH) (*Router, error) {
	r := &Router{
		host:        host,
		busySubnet:  make(map[byte]struct{}),
		busyPhy:     make(map[string]int),
		availIfaces: make(map[string]*netDev),
		busyIfaces:  make(map[string]*netDev),
	}
	if err := r.initialize(ctx); err != nil {
		r.Close(ctx)
		return nil, err
	}

	return r, nil
}

// removeWifiIfaces removed the interfaces listed by "iw dev".
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

const minSpatialStream = 2

// setupWifiPhys lists available phys and enable the antenna.
func (r *Router) setupWifiPhys(ctx context.Context) error {
	if err := r.removeWifiIfaces(ctx); err != nil {
		return err
	}
	iwr := remote_iw.NewRunner(r.host)
	wiphys, err := iwr.ListPhys(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list phys")
	}
	for _, p := range wiphys {
		if r.board == "whirlwind" && (p.RxAntenna < minSpatialStream || p.TxAntenna < minSpatialStream) {
			// We don't want to use the 3rd radio (1x1 auxiliary radio) on Whirlwind.
			continue
		}
		if p.SupportSetAntennaMask() {
			if err := iwr.SetAntennaBitmap(ctx, p.Name, p.TxAntenna, p.RxAntenna); err != nil {
				return errors.Wrapf(err, "failed to set bitmap for %s", p.Name)
			}
		}
		r.phys = append(r.phys, p)
	}
	return nil
}

// initialize prepares initial test AP state (e.g., initializing wiphy/wdev).
func (r *Router) initialize(ctx context.Context) error {
	// Get board information.
	board, err := getHostBoard(ctx, r.host)
	if err != nil {
		return err
	}
	r.board = board

	// Setup working dir.
	if err := r.host.Command("rm", "-rf", r.workDir()).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to remove workdir %q", r.workDir())
	}
	if err := r.host.Command("mkdir", "-p", r.workDir()).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to create workdir %q", r.workDir())
	}

	// Kill remaining hostapd/dnsmasq.
	hostapd.KillAll(ctx, r.host)
	dhcp.KillAll(ctx, r.host)
	// Stop upstart job wpasupplicant if available. (ignore the error as it might be stopped already)
	r.host.Command("stop", "wpasupplicant").Run(ctx)

	// TODO(crbug.com/839164): Verify if we still need to truncate uma-events.

	iwr := remote_iw.NewRunner(r.host)
	// Set default regulatory domain to US.
	if err := iwr.SetRegulatoryDomain(ctx, "US"); err != nil {
		return errors.Wrap(err, "failed to set regulatory domain to US")
	}

	// Set up phys.
	if err := r.setupWifiPhys(ctx); err != nil {
		return err
	}

	// TODO(crbug.com/774808): configure hw_random.

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

// selectPhy finds an suitable phy for the given channel.
func (r *Router) selectPhy(ctx context.Context, channel int) (*iw.Phy, error) {
	// function to check if p1 is better then p2.
	better := func(p1, p2 *iw.Phy) bool {
		if p1 == nil {
			return false
		}
		if p2 == nil {
			return true
		}
		// Prefer free phy.
		if r.busyPhy[p1.Name] == 0 && r.busyPhy[p2.Name] > 0 {
			return true
		}
		return false
	}
	freq, err := hostapd.ChannelToFrequency(channel)
	if err != nil {
		return nil, errors.Errorf("channel %d not available", channel)
	}
	var selected *iw.Phy
	for _, phy := range r.phys {
		// Check channel support.
		supported := false
		for _, b := range phy.Bands {
			if _, ok := b.FrequencyFlags[freq]; ok {
				supported = true
				break
			}
		}
		if !supported {
			continue
		}
		if better(phy, selected) {
			selected = phy
		}
	}
	if selected == nil {
		return nil, errors.Errorf("cannot find supported interface for channel=%d", channel)
	}
	return selected, nil
}

// selectInterface finds an available interface suitable for the given channel and type.
func (r *Router) selectInterface(ctx context.Context, ch int, t ifaceType) (string, error) {
	phy, err := r.selectPhy(ctx, ch)
	if err != nil {
		return "", err
	}
	// reuse from available interface list.
	var selected string
	for ifaceName, nd := range r.availIfaces {
		if nd.phyName == phy.Name && nd.ifaceType == t {
			selected = ifaceName
			break
		}
	}
	// no available interface to reuse, create one.
	if selected == "" {
		var err error
		selected, err = r.createWifiIface(ctx, phy.Name, t)
		if err != nil {
			return "", err
		}
	}
	// TODO(crbug.com/1034875): configure interface for monitor interfaces.
	return selected, nil
}

// getHandleID returns an unique ID string for services on this router. Useful for giving names to daemons/services.
func (r *Router) getHandleID() string {
	id := strconv.Itoa(r.handleID)
	r.handleID++
	return id
}

// StartHostapd starts a hostapd service which includes hostapd and dhcpd. It will select a suitable
// phy automatically. The handle object for the service is returned.
func (r *Router) StartHostapd(ctx context.Context, conf *hostapd.Config) (*InterfaceHandle, error) {
	// Reserve required resources.
	name := r.getHandleID()
	iface, err := r.selectInterface(ctx, conf.Channel, ifaceTypeManaged)
	if err != nil {
		return nil, err
	}
	r.setIfaceBusy(iface)

	idx, err := r.getSubnetIdx()
	if err != nil {
		r.releaseIface(iface)
		return nil, err
	}

	h := &InterfaceHandle{
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
		r.releaseIface(iface)
		return nil, err
	}
	return h, nil
}

// StopHostapd stops a hostapd service.
func (r *Router) StopHostapd(ctx context.Context, h *InterfaceHandle) error {
	r.freeSubnetIdx(h.subnetIdx)
	r.releaseIface(h.iface)
	return h.stop(ctx)
}

// workDir returns the directory to place temporary files on router.
func (r *Router) workDir() string {
	return workingDir
}

// Utilities for resource control.

// removeWifiIface removes the interface with name=iface.
func (r *Router) removeWifiIface(ctx context.Context, iface string) error {
	testing.ContextLogf(ctx, "Deleting wdev %s on router", iface)
	if out, err := r.host.Command("iw", "dev", iface, "del").Output(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete wdev %s: %s", iface, string(out))
	}
	r.releaseIface(iface)
	delete(r.availIfaces, iface)
	return nil
}

// getIfaceName returns an unique name for interface with type t.
func (r *Router) getIfaceName(t ifaceType) string {
	name := fmt.Sprintf("%s%d", string(t), r.ifaceID)
	r.ifaceID++
	return name
}

// createWifiIface creates an interface on phy with type=t and returns the name of created interface.
func (r *Router) createWifiIface(ctx context.Context, phy string, t ifaceType) (string, error) {
	iface := r.getIfaceName(t)
	testing.ContextLogf(ctx, "Creating wdev %s on wiphy %s", iface, phy)
	cmd := r.host.Command("iw", "phy", phy, "interface", "add", iface, "type", string(t))
	if out, err := cmd.Output(ctx); err != nil {
		return "", errors.Wrapf(err, "failed to create wdev %s on wiphy %s: %s", iface, phy, string(out))
	}
	r.availIfaces[iface] = &netDev{
		phyName:   phy,
		ifaceType: t,
	}
	return iface, nil
}

// setIfaceBusy marks iface as busy.
func (r *Router) setIfaceBusy(iface string) {
	nd, ok := r.availIfaces[iface]
	if !ok {
		return
	}
	r.busyIfaces[iface] = nd
	delete(r.availIfaces, iface)
	r.busyPhy[nd.phyName] = r.busyPhy[nd.phyName] + 1
}

// releaseIface marks iface as free.
func (r *Router) releaseIface(iface string) {
	nd, ok := r.busyIfaces[iface]
	if !ok {
		return
	}
	r.availIfaces[iface] = nd
	delete(r.busyIfaces, iface)
	r.busyPhy[nd.phyName] = r.busyPhy[nd.phyName] - 1
}

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

// collectLogs downloads log files from router.
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

// getHostBoard returns the board information on a chromeos host.
// NOTICE: This function is only intended for handling some corner condition
// for router setup. If you're trying to identify specific board of DUT, maybe
// software/hardware dependency is what you want instead of this.
func getHostBoard(ctx context.Context, host *host.SSH) (string, error) {
	const lsbReleasePath = "/etc/lsb-release"
	const crosReleaseBoardKey = "CHROMEOS_RELEASE_BOARD"

	cmd := host.Command("cat", lsbReleasePath)
	out, err := cmd.Output(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read %s", lsbReleasePath)
	}
	for _, line := range strings.Split(string(out), "\n") {
		tokens := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(tokens) != 2 {
			continue
		}
		if tokens[0] == crosReleaseBoardKey {
			return tokens[1], nil
		}
	}
	return "", errors.Errorf("no %s key found in %s", crosReleaseBoardKey, lsbReleasePath)
}
