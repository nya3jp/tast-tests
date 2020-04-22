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
	"time"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	remote_iw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const workingDir = "/tmp/tast-test/"

// Router is used to control an wireless router and stores state of the router.
type Router struct {
	host        *ssh.Conn // TODO(crbug.com/1019537): use a more suitable ssh object.
	name        string
	board       string
	busySubnet  map[byte]struct{}
	phys        map[int]*iw.Phy            // map from phy idx to iw.Phy.
	busyPhy     map[int]map[iw.IfType]bool // map from phy idx to the map of business of interface types.
	availIfaces map[string]*iw.NetDev      // map from interface name to iw.NetDev.
	busyIfaces  map[string]*iw.NetDev      // map from interface name to iw.NetDev.
	ifaceID     int
	iwr         *iw.Runner
}

// NewRouter connects to and initializes the router via SSH then returns the Router object with a shortened context.
// The caller should call r.Close() to perform clean-up. And the shortened context is used to reserve time for the
// clean-up function to run.
func NewRouter(ctx context.Context, host *ssh.Conn, name string) (
	*Router, context.Context, context.CancelFunc, error) {
	ctx, st := timing.Start(ctx, "NewRouter")
	defer st.End()

	r := &Router{
		host:        host,
		name:        name,
		busySubnet:  make(map[byte]struct{}),
		phys:        make(map[int]*iw.Phy),
		busyPhy:     make(map[int]map[iw.IfType]bool),
		availIfaces: make(map[string]*iw.NetDev),
		busyIfaces:  make(map[string]*iw.NetDev),
		iwr:         remote_iw.NewRunner(host),
	}

	shortCtx, shortCtxCancel := ctxutil.Shorten(ctx, 5*time.Second)
	if err := r.initialize(shortCtx); err != nil {
		shortCtxCancel()
		r.Close(ctx)
		return nil, nil, nil, err
	}
	return r, shortCtx, shortCtxCancel, nil
}

// removeWifiIface removes iface with iw command.
func (r *Router) removeWifiIface(ctx context.Context, iface string) error {
	testing.ContextLogf(ctx, "Deleting wdev %s on %s", iface, r.name)
	return r.iwr.RemoveInterface(ctx, iface)
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
	ctx, st := timing.Start(ctx, "setupWifiPhys")
	defer st.End()

	if err := r.removeWifiIfaces(ctx); err != nil {
		return err
	}
	wiphys, err := r.iwr.ListPhys(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list phys")
	}
	if len(wiphys) == 0 {
		return errors.New("Expect at least one wireless phy; found nothing")
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

// configureRNG configures the system's random number generator (RNG)
// to tpm-rng if available.
// Some router, e.g. Gale, uses an inferior RNG as system default RNG,
// which makes hostapd unable to generate high quality keys fast enough.
// Trusted platform module (TPM), if available, should contain a better
// RNG, named tpm-rng. This function tries to switch the system's current
// RNG to tpm-rng if available.
//
// Symptoms of a slow RNG: hostapd complains with:
//
//   WPA: Not enough entropy in random pool to proceed - reject first
//   4-way handshake
//
// Ref:
// https://chromium.googlesource.com/chromiumos/third_party/hostap/+/7ea51f728bb7/src/ap/wpa_auth.c#1854
//
// Linux devices may have RNG parameters at
// /sys/class/misc/hw_random/rng_{available,current}. See:
//   https://www.kernel.org/doc/Documentation/hw_random.txt
func (r *Router) configureRNG(ctx context.Context) error {
	const rngAvailPath = "/sys/class/misc/hw_random/rng_available"
	const rngCurrentPath = "/sys/class/misc/hw_random/rng_current"
	const wantRng = "tpm-rng"

	out, err := r.host.Command("cat", rngCurrentPath).Output(ctx)
	if err != nil {
		// The system might not support hw_random, skip the configuration.
		return nil
	}
	current := strings.TrimSpace(string(out))
	if current == wantRng {
		return nil
	}

	out, err = r.host.Command("cat", rngAvailPath).Output(ctx)
	if err != nil {
		return err
	}

	supported := false
	for _, rng := range strings.Split(strings.TrimSpace(string(out)), " ") {
		if wantRng == rng {
			supported = true
			break
		}
	}
	if !supported {
		return nil
	}

	testing.ContextLogf(ctx, "Switching RNGs: %s -> %s", current, wantRng)
	if err := fileutil.WriteToHostDirect(ctx, r.host, rngCurrentPath, []byte(wantRng)); err != nil {
		return err
	}
	return nil
}

// initialize prepares initial test AP state (e.g., initializing wiphy/wdev).
func (r *Router) initialize(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "initialize")
	defer st.End()

	board, err := hostBoard(ctx, r.host)
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

	killHostapdDhcp := func() {
		ctx, st := timing.Start(ctx, "killHostapdDhcp")
		defer st.End()

		// Kill remaining hostapd/dnsmasq.
		hostapd.KillAll(ctx, r.host)
		dhcp.KillAll(ctx, r.host)
	}
	killHostapdDhcp()

	// TODO(crbug.com/839164): Current CrOS on router haven't got the fix in crrev.com/c/1979112.
	// Let's keep the truncate and remove it after we have router updated.
	const umaEventsPath = "/var/lib/metrics/uma-events"
	if err := r.host.Command("truncate", "-s", "0", "-c", umaEventsPath).Run(ctx); err != nil {
		// Don't return error here, as it might not bother the test as long as it does not
		// fill the whole partition.
		testing.ContextLogf(ctx, "Failed to truncate %s: %v", umaEventsPath, err)
	}

	if err := r.iwr.SetRegulatoryDomain(ctx, "US"); err != nil {
		return errors.Wrap(err, "failed to set regulatory domain to US")
	}

	stopDaemon := func() {
		ctx, st := timing.Start(ctx, "stopDaemon")
		defer st.End()

		// Stop upstart job wpasupplicant if available. (ignore the error as it might be stopped already)
		r.host.Command("stop", "wpasupplicant").Run(ctx)
		// Stop avahi if available as it just causes unnecessary network traffic.
		r.host.Command("stop", "avahi").Run(ctx)
	}
	stopDaemon()

	// Configure hw_random, see function doc for more details.
	if err := r.configureRNG(ctx); err != nil {
		return errors.Wrap(err, "failed to configure hw_random")
	}

	return nil
}

// Close cleans the resource used by Router.
func (r *Router) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "router.Close")
	defer st.End()

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

// phy finds an suitable phy for the given channel and target interface type t.
// The selected phy index is returned.
func (r *Router) phy(ctx context.Context, channel int, t iw.IfType) (int, error) {
	freq, err := hostapd.ChannelToFrequency(channel)
	if err != nil {
		return 0, errors.Errorf("channel %d not available", channel)
	}
	for id, phy := range r.phys {
		// Skip busy phys.
		if r.isPhyBusy(id, t) {
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

// netDev finds an available interface suitable for the given channel and type.
func (r *Router) netDev(ctx context.Context, channel int, t iw.IfType) (*iw.NetDev, error) {
	ctx, st := timing.Start(ctx, "netDev")
	defer st.End()

	phyID, err := r.phy(ctx, channel, t)
	if err != nil {
		return nil, err
	}
	// First check if there's an available interface on target phy.
	for _, nd := range r.availIfaces {
		if nd.PhyNum == phyID && nd.IfType == t {
			return nd, nil
		}
	}
	// No available interface on phy, create one.
	return r.createWifiIface(ctx, phyID, t)
}

// StartAPIface starts a hostapd service which includes hostapd and dhcpd. It will select a suitable
// phy and re-use or create interface on the phy. Name is used on the path to store logs, config files
// or related resources. The handle object for the service is returned.
// Note that after getting an APIface, h, the caller should defer h.StopAPIfaceClose() and use
// h.ReserveForStopAPIface() to reserve time for calling h.StopAPIface()
func (r *Router) StartAPIface(fullCtx context.Context, name string, conf *hostapd.Config) (
	*APIface, context.Context, context.CancelFunc, error) {
	fullCtx, st := timing.Start(fullCtx, "router.StartAPIface")
	defer st.End()

	// Reserve required resources.
	nd, err := r.netDev(fullCtx, conf.Channel, iw.IfTypeManaged)
	if err != nil {
		return nil, nil, nil, err
	}
	iface := nd.IfName
	r.setIfaceBusy(iface)

	idx, err := r.reserveSubnetIdx()
	if err != nil {
		r.freeIface(iface)
		return nil, nil, nil, err
	}

	h := &APIface{
		host:      r.host,
		name:      name,
		iface:     iface,
		workDir:   r.workDir(),
		subnetIdx: idx,
		config:    conf,
	}

	shortCtx, shortCtxCancel, err := h.start(fullCtx)
	if err != nil {
		r.StopAPIface(fullCtx, h)
		return nil, nil, nil, err
	}
	return h, shortCtx, shortCtxCancel, nil
}

// StopAPIface stops the InterfaceHandle, release the subnet and mark the interface
// as free to re-use.
func (r *Router) StopAPIface(ctx context.Context, h *APIface) error {
	ctx, st := timing.Start(ctx, "router.StopAPIface")
	defer st.End()

	err := h.stop(ctx)
	// Free resources even if something went wrong in stop.
	r.freeSubnetIdx(h.subnetIdx)
	r.freeIface(h.iface)
	return err
}

// StartCapture starts a packet capturer.
// Note that after getting a capturer, c, the caller should defer r.StopCapture(ctx, c) and
// use r.ReserveForStopCapture(ctx, c) to reserve time for calling the deferred call.
func (r *Router) StartCapture(fullCtx context.Context, name string, ch int, freqOps []iw.SetFreqOption, pcapOps ...pcap.Option) (
	ret *pcap.Capturer, shortCtx context.Context, shortCtxCancel context.CancelFunc, retErr error) {
	fullCtx, st := timing.Start(fullCtx, "router.StartCapture")
	defer st.End()

	freq, err := hostapd.ChannelToFrequency(ch)
	if err != nil {
		return nil, nil, nil, err
	}

	// Shorten ctx to reserve time for running defer func() handling retErr != nil cases.
	// Note that it shortens ctx twice. We only need to call cancel of the first shortened context
	// because the shorten context's Done channel is closed when the parent context's Done channel is closed.
	sCtx, sCtxCancel := ctxutil.Shorten(fullCtx, time.Second)
	defer func() {
		if retErr != nil {
			sCtxCancel()
		}
	}()

	nd, err := r.netDev(sCtx, ch, iw.IfTypeMonitor)
	if err != nil {
		return nil, nil, nil, err
	}
	iface := nd.IfName
	shared := r.isPhyBusyAny(nd.PhyNum)

	r.setIfaceBusy(iface)
	defer func() {
		if retErr != nil {
			r.freeIface(iface)
		}
	}()

	if err := r.host.Command("ip", "link", "set", iface, "up").Run(sCtx); err != nil {
		return nil, nil, nil, errors.Wrapf(err, "failed to set %s up", iface)
	}
	defer func() {
		if retErr != nil {
			if err := r.host.Command("ip", "link", "set", iface, "down").Run(fullCtx); err != nil {
				testing.ContextLogf(fullCtx, "Failed to set %s down, err=%s", iface, err.Error())
			}
		}
	}()

	if !shared {
		// The interface is not shared, set up frequency and bandwidth.
		if err := r.iwr.SetFreq(sCtx, iface, freq, freqOps...); err != nil {
			return nil, nil, nil, errors.Wrapf(err, "failed to set frequency for interface %s", iface)
		}
	} else {
		testing.ContextLogf(sCtx, "Skip configuring of the shared interface %s", iface)
	}

	c, sCtx2, _, err := pcap.StartCapturer(sCtx, r.host, name, iface, r.workDir(), pcapOps...)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to start a packet capturer")
	}
	return c, sCtx2, sCtxCancel, nil
}

// StopCapture stops the packet capturer and releases related resources.
func (r *Router) StopCapture(ctx context.Context, capturer *pcap.Capturer) error {
	ctx, st := timing.Start(ctx, "router.StopCapture")
	defer st.End()

	var firstErr error
	collectErr := func(err error) {
		if firstErr != nil {
			testing.ContextLog(ctx, "StopCapture error: ", err)
		} else {
			firstErr = err
		}
	}
	iface := capturer.Interface()
	if err := capturer.Close(ctx); err != nil {
		collectErr(errors.Wrap(err, "failed to stop capturer"))
	}
	if err := r.host.Command("ip", "link", "set", iface, "down").Run(ctx); err != nil {
		collectErr(errors.Wrapf(err, "failed to set %s down", iface))
	}
	r.freeIface(iface)
	return firstErr
}

// workDir returns the directory to place temporary files on router.
func (r *Router) workDir() string {
	return workingDir
}

// Utilities for resource control.

// uniqueIfaceName returns an unique name for interface with type t.
func (r *Router) uniqueIfaceName(t iw.IfType) string {
	name := fmt.Sprintf("%s%d", string(t), r.ifaceID)
	r.ifaceID++
	return name
}

// createWifiIface creates an interface on phy with type=t and returns the name of created interface.
func (r *Router) createWifiIface(ctx context.Context, phyID int, t iw.IfType) (*iw.NetDev, error) {
	ctx, st := timing.Start(ctx, "createWifiIface")
	defer st.End()

	iface := r.uniqueIfaceName(t)
	phy := r.phys[phyID].Name
	testing.ContextLogf(ctx, "Creating wdev %s on wiphy %s", iface, phy)
	if err := r.iwr.AddInterface(ctx, phy, iface, t); err != nil {
		return nil, err
	}
	nd := &iw.NetDev{
		PhyNum: phyID,
		IfName: iface,
		IfType: t,
	}
	r.availIfaces[iface] = nd
	return nd, nil
}

// isPhyBusyAny returns if the phyID is occupied by a busy interface of any type.
func (r *Router) isPhyBusyAny(phyID int) bool {
	if m, ok := r.busyPhy[phyID]; ok {
		for _, b := range m {
			if b {
				return true
			}
		}
	}
	return false
}

// isPhyBusy returns if the phyID is occupied by a busy interface of type t.
func (r *Router) isPhyBusy(phyID int, t iw.IfType) bool {
	m, ok := r.busyPhy[phyID]
	if !ok {
		// Unseen phyID: not busy.
		return false
	}
	// Unseen type has zero value = false: not busy.
	return m[t]
}

// setPhyBusyBool is the internal setter for setPhyBusy and freePhy.
func (r *Router) setPhyBusyBool(phyID int, t iw.IfType, busy bool) {
	m, ok := r.busyPhy[phyID]
	if !ok {
		m = make(map[iw.IfType]bool)
		r.busyPhy[phyID] = m
	}
	m[t] = busy
}

// setPhyBusy marks the phy as occupied by a busy interface of type t.
func (r *Router) setPhyBusy(phyID int, t iw.IfType) {
	r.setPhyBusyBool(phyID, t, true)
}

// freePhy marks phyID as free for interface of type t.
func (r *Router) freePhy(phyID int, t iw.IfType) {
	r.setPhyBusyBool(phyID, t, false)
}

// setIfaceBusy marks iface as busy.
func (r *Router) setIfaceBusy(iface string) {
	nd, ok := r.availIfaces[iface]
	if !ok {
		return
	}
	r.busyIfaces[iface] = nd
	delete(r.availIfaces, iface)
	r.setPhyBusy(nd.PhyNum, nd.IfType)
}

// freeIface marks iface as free.
func (r *Router) freeIface(iface string) {
	nd, ok := r.busyIfaces[iface]
	if !ok {
		return
	}
	r.availIfaces[iface] = nd
	delete(r.busyIfaces, iface)
	r.freePhy(nd.PhyNum, nd.IfType)
}

// reserveSubnetIdx finds a free subnet index and reserves it.
func (r *Router) reserveSubnetIdx() (byte, error) {
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
	ctx, st := timing.Start(ctx, "collectLogs")
	defer st.End()

	collect := map[string]string{
		"/var/log/messages": fmt.Sprintf("debug/%s_host_messages", r.name),
	}
	// TODO(crbug.com/1034875): Trim logs before creation of this object.
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
		if err := linuxssh.GetFile(ctx, r.host, s, dst); err != nil {
			return errors.Wrapf(err, "failed to download %s to %s", s, dst)
		}
	}
	return nil
}

// hostBoard returns the board information on a chromeos host.
// NOTICE: This function is only intended for handling some corner condition
// for router setup. If you're trying to identify specific board of DUT, maybe
// software/hardware dependency is what you want instead of this.
func hostBoard(ctx context.Context, host *ssh.Conn) (string, error) {
	ctx, st := timing.Start(ctx, "hostBoard")
	defer st.End()

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
