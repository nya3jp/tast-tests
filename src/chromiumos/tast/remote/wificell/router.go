// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/network/ip"
	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	remote_ip "chromiumos/tast/remote/network/ip"
	remote_iw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/log"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const workingDir = "/tmp/tast-test/"

// Router is used to control an wireless router and stores state of the router.
type Router struct {
	host          *ssh.Conn
	name          string
	board         string
	busySubnet    map[byte]struct{}
	phys          map[int]*iw.Phy            // map from phy idx to iw.Phy.
	busyPhy       map[int]map[iw.IfType]bool // map from phy idx to the map of business of interface types.
	availIfaces   map[string]*iw.NetDev      // map from interface name to iw.NetDev.
	busyIfaces    map[string]*iw.NetDev      // map from interface name to iw.NetDev.
	ifaceID       int
	iwr           *iw.Runner
	ipr           *ip.Runner
	logCollectors map[string]*log.Collector // map from log path to its collector.
}

// NewRouter connects to and initializes the router via SSH then returns the Router object.
// This method takes two context: ctx and daemonCtx, the first is the context for the New
// method and daemonCtx is for the spawned background daemons.
// After getting a Server instance, d, the caller should call r.Close() at the end, and use the
// shortened ctx (provided by d.ReserveForClose()) before r.Close() to reserve time for it to run.
func NewRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string) (*Router, error) {
	ctx, st := timing.Start(ctx, "NewRouter")
	defer st.End()
	r := &Router{
		host:          host,
		name:          name,
		busySubnet:    make(map[byte]struct{}),
		phys:          make(map[int]*iw.Phy),
		busyPhy:       make(map[int]map[iw.IfType]bool),
		availIfaces:   make(map[string]*iw.NetDev),
		busyIfaces:    make(map[string]*iw.NetDev),
		iwr:           remote_iw.NewRemoteRunner(host),
		ipr:           remote_ip.NewRemoteRunner(host),
		logCollectors: make(map[string]*log.Collector),
	}

	shortCtx, cancel := r.ReserveForClose(ctx)
	defer cancel()
	if err := r.initialize(shortCtx, daemonCtx); err != nil {
		r.Close(ctx)
		return nil, err
	}
	return r, nil
}

// ReserveForClose returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.Close() to reserve time for it to run.
func (r *Router) ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 5*time.Second)
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
// ctx is the deadline for the step and daemonCtx is the lifetime for background
// daemons.
func (r *Router) initialize(ctx, daemonCtx context.Context) error {
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

	// Start log collectors with daemonCtx as it should live longer than current
	// stage when we are in precondition.
	if err := r.startLogCollectors(daemonCtx); err != nil {
		return errors.Wrap(err, "failed to start loggers")
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

	var firstErr error

	// Remove the interfaces that we created.
	for _, nd := range r.availIfaces {
		if err := r.removeWifiIface(ctx, nd.IfName); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove interfaces"))
		}
	}
	for _, nd := range r.busyIfaces {
		testing.ContextLogf(ctx, "iface %s not yet freed", nd.IfName)
		if err := r.removeWifiIface(ctx, nd.IfName); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove interfaces"))
		}
	}
	// Collect closing log to facilitate debugging for error occurs in
	// r.initialize() or after r.CollectLogs().
	if err := r.collectLogs(ctx, ".close"); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to collect logs"))
	}
	if err := r.stopLogCollectors(ctx); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop loggers"))
	}
	if err := r.host.Command("rm", "-rf", r.workDir()).Run(ctx); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove working dir"))
	}
	return firstErr
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
	return r.netDevWithPhyID(ctx, phyID, t)
}

// netDevWithPhyID finds an available interface on phy#phyID and with given type.
func (r *Router) netDevWithPhyID(ctx context.Context, phyID int, t iw.IfType) (*iw.NetDev, error) {
	// First check if there's an available interface on target phy.
	for _, nd := range r.availIfaces {
		if nd.PhyNum == phyID && nd.IfType == t {
			return nd, nil
		}
	}
	// No available interface on phy, create one.
	return r.createWifiIface(ctx, phyID, t)
}

// monitorOnInterface finds an available monitor type interface on the same phy as a
// busy interface with name=iface.
func (r *Router) monitorOnInterface(ctx context.Context, iface string) (*iw.NetDev, error) {
	var ndev *iw.NetDev
	// Find phy ID of iface.
	for name, nd := range r.busyIfaces {
		if name == iface {
			ndev = nd
			break
		}
	}
	if ndev == nil {
		return nil, errors.Errorf("cannot find busy interface %s", iface)
	}
	phyID := ndev.PhyNum
	return r.netDevWithPhyID(ctx, phyID, iw.IfTypeMonitor)
}

// StartAPIface starts a hostapd service which includes hostapd and dhcpd. It will select a suitable
// phy and re-use or create interface on the phy. Name is used on the path to store logs, config files
// or related resources. The handle object for the service is returned.
// After getting an APIface instance, h, the caller should call h.StopAPIfaceClose() at the end,
// and use the shortened ctx (provided by h.ReserveForStopAPIface()) before h.StopAPIfaceClose()
// to reserve time for it to run.
func (r *Router) StartAPIface(ctx context.Context, name string, conf *hostapd.Config) (*APIface, error) {
	ctx, st := timing.Start(ctx, "router.StartAPIface")
	defer st.End()

	// Reserve required resources.
	nd, err := r.netDev(ctx, conf.Channel, iw.IfTypeManaged)
	if err != nil {
		return nil, err
	}
	iface := nd.IfName
	r.setIfaceBusy(iface)

	idx, err := r.reserveSubnetIdx()
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

	// Note that we don't need to reserve time for clean up as h.start() reserves time to clean
	// up itself and the rest of cleaning up in r.StopAPIface() does not limited by ctx.
	if err := h.start(ctx); err != nil {
		r.StopAPIface(ctx, h)
		return nil, err
	}
	return h, nil
}

// ReserveForStopAPIface returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.StopAPIface() to reserve time for it to run.
func (r *Router) ReserveForStopAPIface(ctx context.Context, h *APIface) (context.Context, context.CancelFunc) {
	return h.reserveForStop(ctx)
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
// After getting a Capturer instance, c, the caller should call r.StopCapture(ctx, c) at the end,
// and use the shortened ctx (provided by r.ReserveForStopCapture(ctx, c)) before r.StopCapture()
// to reserve time for it to run.
func (r *Router) StartCapture(ctx context.Context, name string, ch int, freqOps []iw.SetFreqOption, pcapOps ...pcap.Option) (ret *pcap.Capturer, retErr error) {
	ctx, st := timing.Start(ctx, "router.StartCapture")
	defer st.End()

	freq, err := hostapd.ChannelToFrequency(ch)
	if err != nil {
		return nil, err
	}

	nd, err := r.netDev(ctx, ch, iw.IfTypeMonitor)
	if err != nil {
		return nil, err
	}
	iface := nd.IfName
	shared := r.isPhyBusyAny(nd.PhyNum)

	r.setIfaceBusy(iface)
	defer func() {
		if retErr != nil {
			r.freeIface(iface)
		}
	}()

	if err := r.ipr.SetLinkUp(ctx, iface); err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			if err := r.ipr.SetLinkDown(ctx, iface); err != nil {
				testing.ContextLogf(ctx, "Failed to set %s down, err=%s", iface, err.Error())
			}
		}
	}()

	if !shared {
		// The interface is not shared, set up frequency and bandwidth.
		if err := r.iwr.SetFreq(ctx, iface, freq, freqOps...); err != nil {
			return nil, errors.Wrapf(err, "failed to set frequency for interface %s", iface)
		}
	} else {
		testing.ContextLogf(ctx, "Skip configuring of the shared interface %s", iface)
	}

	c, err := pcap.StartCapturer(ctx, r.host, name, iface, r.workDir(), pcapOps...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start a packet capturer")
	}
	return c, nil
}

// ReserveForStopCapture returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.StopCapture() to reserve time for it to run.
func (r *Router) ReserveForStopCapture(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc) {
	return capturer.ReserveForClose(ctx)
}

// StopCapture stops the packet capturer and releases related resources.
func (r *Router) StopCapture(ctx context.Context, capturer *pcap.Capturer) error {
	ctx, st := timing.Start(ctx, "router.StopCapture")
	defer st.End()

	var firstErr error
	iface := capturer.Interface()
	if err := capturer.Close(ctx); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop capturer"))
	}
	if err := r.ipr.SetLinkDown(ctx, iface); err != nil {
		collectFirstErr(ctx, &firstErr, err)
	}
	r.freeIface(iface)
	return firstErr
}

// NewFrameSender creates a frame sender object.
func (r *Router) NewFrameSender(ctx context.Context, iface string) (ret *framesender.Sender, retErr error) {
	nd, err := r.monitorOnInterface(ctx, iface)
	if err != nil {
		return nil, err
	}
	r.setIfaceBusy(nd.IfName)
	defer func() {
		if retErr != nil {
			r.freeIface(nd.IfName)
		}
	}()

	if err := r.cloneMAC(ctx, nd.IfName, iface); err != nil {
		return nil, errors.Wrap(err, "failed to clone MAC")
	}
	if err := r.ipr.SetLinkUp(ctx, nd.IfName); err != nil {
		return nil, err
	}
	return framesender.New(r.host, nd.IfName, r.workDir()), nil
}

// ReserveForCloseFrameSender returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.CloseFrameSender() to reserve
// time for it to run.
func (r *Router) ReserveForCloseFrameSender(ctx context.Context) (context.Context, context.CancelFunc) {
	// FrameSender don't need close, but we still need some time for freeing interface.
	return ctxutil.Shorten(ctx, 2*time.Second)
}

// CloseFrameSender closes frame sender and releases related resources.
func (r *Router) CloseFrameSender(ctx context.Context, s *framesender.Sender) error {
	err := r.ipr.SetLinkDown(ctx, s.Interface())
	r.freeIface(s.Interface())
	return err
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

// logsToCollect is the list of files on router to collect.
var logsToCollect = []string{
	"/var/log/messages",
}

// startLogCollectors starts log collectors.
func (r *Router) startLogCollectors(ctx context.Context) error {
	for _, p := range logsToCollect {
		logger, err := log.StartCollector(ctx, r.host, p)
		if err != nil {
			return errors.Wrap(err, "failed to start log collector")
		}
		r.logCollectors[p] = logger
	}
	return nil
}

// collectLogs downloads log files from router to $OutDir/debug/$r.name with suffix
// appended to the filenames.
func (r *Router) collectLogs(ctx context.Context, suffix string) error {
	ctx, st := timing.Start(ctx, "collectLogs")
	defer st.End()

	baseDir := filepath.Join("debug", r.name)

	for _, src := range logsToCollect {
		dst := filepath.Join(baseDir, filepath.Base(src)+suffix)
		collector := r.logCollectors[src]
		if collector == nil {
			testing.ContextLogf(ctx, "No log collector for %s found", src)
			continue
		}
		f, err := fileutil.PrepareOutDirFile(ctx, dst)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to collect %q, err: %v", src, err)
			continue
		}
		if err := collector.Dump(f); err != nil {
			testing.ContextLogf(ctx, "Failed to dump %q logs, err: %v", src, err)
			continue

		}
	}
	return nil
}

// stopLogCollectors closes all log collectors spawned.
func (r *Router) stopLogCollectors(ctx context.Context) error {
	var firstErr error
	for _, c := range r.logCollectors {
		if err := c.Close(ctx); err != nil {
			collectFirstErr(ctx, &firstErr, err)
		}
	}
	return firstErr
}

// cloneMAC clones the MAC address of src to dst.
func (r *Router) cloneMAC(ctx context.Context, dst, src string) error {
	mac, err := r.ipr.MAC(ctx, src)
	if err != nil {
		return err
	}
	return r.ipr.SetMAC(ctx, dst, mac)
}

// CollectLogs downloads log files from router to OutDir.
func (r *Router) CollectLogs(ctx context.Context) error {
	return r.collectLogs(ctx, "")
}

// SetAPIfaceDown brings down the interface that the APIface uses.
func (r *Router) SetAPIfaceDown(ctx context.Context, iface string) error {
	if err := r.ipr.SetLinkDown(ctx, iface); err != nil {
		return errors.Wrapf(err, "failed to set %s down", iface)
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

// ChangeAPIfaceSubnetIdx restarts the dhcp server with a different subnet index.
// Note that a call of StopAPIface is still needed on failure.
func (r *Router) ChangeAPIfaceSubnetIdx(ctx context.Context, h *APIface) (retErr error) {
	oldIdx := h.subnetIdx
	newIdx, err := r.reserveSubnetIdx()
	if err != nil {
		return errors.Wrap(err, "failed to reserve a new subnet index")
	}
	defer func() {
		// On failure, the subnetIdx of h will not change so we should free the new
		// index here and let the old index be freed in the future StopAPIface call.
		if retErr != nil {
			r.freeSubnetIdx(newIdx)
		} else {
			r.freeSubnetIdx(oldIdx)
		}
	}()

	testing.ContextLogf(ctx, "changing AP subnet index from %d to %d", oldIdx, newIdx)
	return h.changeSubnetIdx(ctx, newIdx)
}
