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

const (
	ifaceTypeManaged = "managed"
)

// Router is used to control an wireless router and stores state of the router.
type Router struct {
	host        *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	board       string
	busySubnet  map[byte]struct{}
	phys        map[int]*iw.Phy       // map from phy idx to iw.Phy.
	busyPhy     map[int]int           // map from phy idx to busy interface count.
	availIfaces map[string]*iw.NetDev // map from interface name to iw.NetDev.
	busyIfaces  map[string]*iw.NetDev // map from interface name to iw.NetDev.
	handleID    int
	ifaceID     int
	iwr         *iw.Runner
}

// NewRouter connects to and initializes the router via SSH then returns the Router object.
func NewRouter(ctx context.Context, host *host.SSH) (*Router, error) {
	r := &Router{
		host:        host,
		busySubnet:  make(map[byte]struct{}),
		phys:        make(map[int]*iw.Phy),
		busyPhy:     make(map[int]int),
		availIfaces: make(map[string]*iw.NetDev),
		busyIfaces:  make(map[string]*iw.NetDev),
		iwr:         remote_iw.NewRunner(host),
	}
	if err := r.initialize(ctx); err != nil {
		r.Close(ctx)
		return nil, err
	}

	return r, nil
}

// removeWifiIface removes iface with iw command.
func (r *Router) removeWifiIface(ctx context.Context, iface string) error {
	testing.ContextLogf(ctx, "Deleting wdev %s on router", iface)
	// TODO(crbug.com/1034875): move iw operations into iw_runner.
	if out, err := r.host.Command("iw", "dev", iface, "del").CombinedOutput(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete wdev %s: %q", iface, string(out))
	}
	return nil
}

// removeWifiIfaces removes all WiFi interfaces.
func (r *Router) removeWifiIfaces(ctx context.Context) error {
	wdevs, err := r.iwr.ListInterfaces(ctx)
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

// setupWifiPhys fills r.phys and enables their antennas.
func (r *Router) setupWifiPhys(ctx context.Context) error {
	if err := r.removeWifiIfaces(ctx); err != nil {
		return err
	}
	wiphys, err := r.iwr.ListPhys(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list phys")
	}
	// isWhirlwindAuxPhy returns true if p is Whirlwind's 1x1 auxiliary radio.
	isWhirlwindAuxPhy := func(p *iw.Phy) bool {
		return r.board == "whirlwind" && (p.RxAntenna < 2 || p.TxAntenna < 2)
	}
	for _, p := range wiphys {
		if isWhirlwindAuxPhy(p) {
			// We don't want to use the 3rd radio (1x1 auxiliary radio) on Whirlwind.
			continue
		}
		if p.SupportSetAntennaMask() {
			if err := r.iwr.SetAntennaBitmap(ctx, p.Name, p.TxAntenna, p.RxAntenna); err != nil {
				return errors.Wrapf(err, "failed to set bitmap for %s", p.Name)
			}
		}
		phyIDBytes, err := r.host.Command("cat", fmt.Sprintf("/sys/class/ieee80211/%s/index", p.Name)).Output(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get phy idx for %s", p.Name)
		}
		phyID, err := strconv.Atoi(strings.TrimSpace(string(phyIDBytes)))
		if err != nil {
			return errors.Wrapf(err, "invalid phy idx %s", string(phyIDBytes))
		}
		r.phys[phyID] = p
	}
	return nil
}

// initialize prepares initial test AP state (e.g., initializing wiphy/wdev).
func (r *Router) initialize(ctx context.Context) error {
	board, err := getHostBoard(ctx, r.host)
	if err != nil {
		return err
	}
	r.board = board

	// Set up working dir.
	if err := r.host.Command("rm", "-rf", r.workDir()).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to remove workdir %q", r.workDir())
	}
	if err := r.host.Command("mkdir", "-p", r.workDir()).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to create workdir %q", r.workDir())
	}

	if err := r.setupWifiPhys(ctx); err != nil {
		return err
	}

	// Kill remaining hostapd/dnsmasq.
	hostapd.KillAll(ctx, r.host)
	dhcp.KillAll(ctx, r.host)

	// TODO(crbug.com/839164): Verify if we still need to truncate uma-events.

	if err := r.iwr.SetRegulatoryDomain(ctx, "US"); err != nil {
		return errors.Wrap(err, "failed to set regulatory domain to US")
	}

	// Stop upstart job wpasupplicant if available. (ignore the error as it might be stopped already)
	r.host.Command("stop", "wpasupplicant").Run(ctx)

	// TODO(crbug.com/774808): configure hw_random.

	return nil
}

// Close cleans the resource used by Router.
func (r *Router) Close(ctx context.Context) error {
	var err error
	// Remove the interfaces that we created.
	for _, nd := range r.availIfaces {
		if err2 := r.removeWifiIface(ctx, nd.IfName); err2 != nil {
			err = errors.Wrapf(err, "failed to remove interfaces, err=%s", err2.Error())
		}
	}
	for _, nd := range r.busyIfaces {
		testing.ContextLogf(ctx, "iface %s not yet freed", nd.IfName)
		if err2 := r.removeWifiIface(ctx, nd.IfName); err2 != nil {
			err = errors.Wrapf(err, "failed to remove interfaces, err=%s", err2.Error())
		}
	}
	if err2 := r.collectLogs(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to collect logs, err=%s", err2.Error())
	}
	if err2 := r.host.Command("rm", "-rf", r.workDir()).Run(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to remove working dir, err=%s", err2.Error())
	}
	return err
}

// selectPhy finds an suitable phy for the given channel and returns its phy index.
func (r *Router) selectPhy(ctx context.Context, channel int) (int, error) {
	freq, err := hostapd.ChannelToFrequency(channel)
	if err != nil {
		return 0, errors.Errorf("channel %d not available", channel)
	}
	for id, phy := range r.phys {
		// Skip busy phys.
		if r.busyPhy[id] > 0 {
			continue
		}
		// Check channel support.
		for _, b := range phy.Bands {
			if _, ok := b.FrequencyFlags[freq]; ok {
				return id, nil
			}
		}
	}
	return 0, errors.Errorf("cannot find supported phy for channel=%d", channel)
}

// selectInterface finds an available interface suitable for the given channel and type.
func (r *Router) selectInterface(ctx context.Context, channel int, t string) (string, error) {
	phyID, err := r.selectPhy(ctx, channel)
	if err != nil {
		return "", err
	}
	// First check if there's an available interface on target phy.
	var selected string
	for ifaceName, nd := range r.availIfaces {
		if nd.PhyNum == phyID && nd.IfType == t {
			selected = ifaceName
			break
		}
	}
	// No available interface on phy, create one.
	if selected == "" {
		var err error
		selected, err = r.createWifiIface(ctx, phyID, t)
		if err != nil {
			return "", err
		}
	}
	// TODO(crbug.com/1034875): configure interface for monitor interfaces.
	return selected, nil
}

// getUniqueServiceName returns an unique ID string for services on this router. Useful for giving names to daemons/services.
func (r *Router) getUniqueServiceName() string {
	id := strconv.Itoa(r.handleID)
	r.handleID++
	return id
}

// StartAPIface starts a hostapd service which includes hostapd and dhcpd. It will select a suitable
// phy and re-use or create interface on the phy. The handle object for the service is returned.
func (r *Router) StartAPIface(ctx context.Context, conf *hostapd.Config) (*APIface, error) {
	// Reserve required resources.
	name := r.getUniqueServiceName()
	iface, err := r.selectInterface(ctx, conf.Channel, ifaceTypeManaged)
	if err != nil {
		return nil, err
	}
	r.setIfaceBusy(iface)

	idx, err := r.getSubnetIdx()
	if err != nil {
		r.freeIface(iface)
		return nil, err
	}

	h := &APIface{
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
		r.freeIface(iface)
		return nil, err
	}
	return h, nil
}

// StopAPIface stops the InterfaceHandle, release the subnet and mark the interface
// as free to re-use.
func (r *Router) StopAPIface(ctx context.Context, h *APIface) error {
	err := h.stop(ctx)
	// Free resources even if something went wrong in stop.
	r.freeSubnetIdx(h.subnetIdx)
	r.freeIface(h.iface)
	return err
}

// workDir returns the directory to place temporary files on router.
func (r *Router) workDir() string {
	return workingDir
}

// Utilities for resource control.

// getUniqueIfaceName returns an unique name for interface with type t.
func (r *Router) getUniqueIfaceName(t string) string {
	name := fmt.Sprintf("%s%d", t, r.ifaceID)
	r.ifaceID++
	return name
}

// createWifiIface creates an interface on phy with type=t and returns the name of created interface.
func (r *Router) createWifiIface(ctx context.Context, phyID int, t string) (string, error) {
	iface := r.getUniqueIfaceName(t)
	phy := r.phys[phyID].Name
	testing.ContextLogf(ctx, "Creating wdev %s on wiphy %s", iface, phy)
	// TODO(crbug.com/1034875): move iw operations into iw_runner.
	cmd := r.host.Command("iw", "phy", phy, "interface", "add", iface, "type", string(t))
	if out, err := cmd.CombinedOutput(ctx); err != nil {
		return "", errors.Wrapf(err, "failed to create wdev %s on wiphy %s: %q", iface, phy, string(out))
	}
	r.availIfaces[iface] = &iw.NetDev{
		PhyNum: phyID,
		IfName: iface,
		IfType: t,
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
	r.busyPhy[nd.PhyNum]++
}

// freeIface marks iface as free.
func (r *Router) freeIface(iface string) {
	nd, ok := r.busyIfaces[iface]
	if !ok {
		return
	}
	r.availIfaces[iface] = nd
	delete(r.busyIfaces, iface)
	r.busyPhy[nd.PhyNum]--
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
