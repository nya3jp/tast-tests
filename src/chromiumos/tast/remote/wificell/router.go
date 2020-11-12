// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"fmt"
	"net"
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

const (
	// Autotest may be used on these routers too, and if it failed to clean up, we may be out of space in /tmp.
	autotestWorkdirGlob = "/tmp/autotest-*"
	workingDir          = "/tmp/tast-test/"
)

const (
	// NOTE: shill does not manage (i.e., run a dhcpcd on) the device with prefix "veth".
	// See kIgnoredDeviceNamePrefixes in http://cs/chromeos_public/src/platform2/shill/device_info.cc
	vethPrefix     = "vethA"
	vethPeerPrefix = "vethB"
	bridgePrefix   = "tastbr"
)

// Router is used to control an wireless router and stores state of the router.
type Router struct {
	host          *ssh.Conn
	name          string
	board         string
	phys          map[int]*iw.Phy       // map from phy idx to iw.Phy.
	availIfaces   map[string]*iw.NetDev // map from interface name to iw.NetDev.
	busyIfaces    map[string]*iw.NetDev // map from interface name to iw.NetDev.
	ifaceID       int
	bridgeID      int
	vethID        int
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
		phys:          make(map[int]*iw.Phy),
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

	// Clean up Autotest working dir, in case we're out of space.
	// NB: we need 'sh' to handle the glob.
	if err := r.host.Command("sh", "-c", strings.Join([]string{"rm", "-rf", autotestWorkdirGlob}, " ")).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to remove workdir %q", autotestWorkdirGlob)
	}

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
	if err := r.removeDevicesWithPrefix(ctx, bridgePrefix); err != nil {
		return err
	}
	// Note that we only need to remove one side of each veth pair.
	if err := r.removeDevicesWithPrefix(ctx, vethPrefix); err != nil {
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

	if err := r.removeDevicesWithPrefix(ctx, bridgePrefix); err != nil {
		return err
	}
	// Note that we only need to remove one side of each veth pair.
	if err := r.removeDevicesWithPrefix(ctx, vethPrefix); err != nil {
		return err
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
	// Try to find an idle phy which is suitable
	for id, phy := range r.phys {
		if r.isPhyBusy(id, t) {
			continue
		}
		if phySupportsFrequency(phy, freq) {
			return id, nil
		}
	}
	// Try to find any phy which is suitable, even a busy one
	for id, phy := range r.phys {
		if phySupportsFrequency(phy, freq) {
			return id, nil
		}
	}
	return 0, errors.Errorf("cannot find supported phy for channel=%d", channel)
}

// phySupportsFrequency returns true if any band of the given phy supports
// the desired frequency.
func phySupportsFrequency(phy *iw.Phy, freq int) bool {
	for _, b := range phy.Bands {
		if _, ok := b.FrequencyFlags[freq]; ok {
			return true
		}
	}
	return false
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

// StartHostapd starts the hostapd server.
func (r *Router) StartHostapd(ctx context.Context, name string, conf *hostapd.Config) (_ *hostapd.Server, retErr error) {
	if err := conf.SecurityConfig.InstallRouterCredentials(ctx, r.host, r.workDir()); err != nil {
		return nil, errors.Wrap(err, "failed to install router credentials")
	}

	nd, err := r.netDev(ctx, conf.Channel, iw.IfTypeManaged)
	if err != nil {
		return nil, err
	}
	iface := nd.IfName
	r.setIfaceBusy(iface)
	defer func() {
		if retErr != nil {
			r.freeIface(iface)
		}
	}()
	return r.startHostapdOnIface(ctx, iface, name, conf)
}

func (r *Router) startHostapdOnIface(ctx context.Context, iface, name string, conf *hostapd.Config) (_ *hostapd.Server, retErr error) {
	hs, err := hostapd.StartServer(ctx, r.host, name, iface, r.workDir(), conf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start hostapd server")
	}
	defer func(ctx context.Context) {
		if retErr != nil {
			if err := hs.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to stop hostapd server while StartHostapd has failed: ", err)
			}
		}
	}(ctx)
	ctx, cancel := hs.ReserveForClose(ctx)
	defer cancel()

	if err := r.iwr.SetTxPowerAuto(ctx, iface); err != nil {
		return nil, errors.Wrap(err, "failed to set txpower to auto")
	}
	return hs, nil
}

// StopHostapd stops the hostapd server.
func (r *Router) StopHostapd(ctx context.Context, hs *hostapd.Server) error {
	var firstErr error
	iface := hs.Interface()
	if err := hs.Close(ctx); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop hostapd"))
	}
	collectFirstErr(ctx, &firstErr, r.ipr.SetLinkDown(ctx, iface))
	r.freeIface(iface)
	return firstErr
}

// ReconfigureHostapd restarts the hostapd server with the new config. It preserves the interface and the name of the old hostapd server.
func (r *Router) ReconfigureHostapd(ctx context.Context, hs *hostapd.Server, conf *hostapd.Config) (_ *hostapd.Server, retErr error) {
	iface := hs.Interface()
	name := hs.Name()
	if err := r.StopHostapd(ctx, hs); err != nil {
		return nil, errors.Wrap(err, "failed to stop hostapd server")
	}
	r.setIfaceBusy(iface)
	defer func() {
		if retErr != nil {
			r.freeIface(iface)
		}
	}()
	return r.startHostapdOnIface(ctx, iface, name, conf)
}

// StartDHCP starts the DHCP server and configures the server IP.
func (r *Router) StartDHCP(ctx context.Context, name, iface string, ipStart, ipEnd, serverIP, broadcastIP net.IP, mask net.IPMask) (_ *dhcp.Server, retErr error) {
	if err := r.ipr.FlushIP(ctx, iface); err != nil {
		return nil, err
	}
	maskLen, _ := mask.Size()
	if err := r.ipr.AddIP(ctx, iface, serverIP, maskLen, ip.AddIPBroadcast(broadcastIP)); err != nil {
		return nil, err
	}
	defer func(ctx context.Context) {
		if retErr != nil {
			if err := r.ipr.FlushIP(ctx, iface); err != nil {
				testing.ContextLogf(ctx, "Failed to flush the interface %s while StartDHCP has failed: %v", iface, err)
			}
		}
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()
	ds, err := dhcp.StartServer(ctx, r.host, name, iface, r.workDir(), ipStart, ipEnd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start DHCP server")
	}
	return ds, nil
}

// StopDHCP stops the DHCP server and flushes the interface.
func (r *Router) StopDHCP(ctx context.Context, ds *dhcp.Server) error {
	var firstErr error
	iface := ds.Interface()
	if err := ds.Close(ctx); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop dhcpd"))
	}
	collectFirstErr(ctx, &firstErr, r.ipr.FlushIP(ctx, iface))
	return firstErr
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

// StartRawCapturer starts a capturer on an existing interface on the router instead of a
// monitor type interface.
// This function is useful for the tests that don't care the 802.11 frames but the behavior
// of upper layer traffic and tests can capture packets directly on AP's interface.
func (r *Router) StartRawCapturer(ctx context.Context, name, iface string, ops ...pcap.Option) (*pcap.Capturer, error) {
	return pcap.StartCapturer(ctx, r.host, name, iface, r.workDir(), ops...)
}

// ReserveForStopRawCapturer returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.StopRawCapture to reserve time for it.
func (r *Router) ReserveForStopRawCapturer(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc) {
	return capturer.ReserveForClose(ctx)
}

// StopRawCapturer stops the packet capturer (no extra resources to release).
func (r *Router) StopRawCapturer(ctx context.Context, capturer *pcap.Capturer) error {
	return capturer.Close(ctx)
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

// NewBridge returns a bridge name for tests to use. Note that the caller is responsible to call ReleaseBridge.
func (r *Router) NewBridge(ctx context.Context) (_ string, retErr error) {
	br := fmt.Sprintf("%s%d", bridgePrefix, r.bridgeID)
	r.bridgeID++
	if err := r.ipr.AddLink(ctx, br, "bridge"); err != nil {
		return "", err
	}
	defer func() {
		if retErr != nil {
			if err := r.ipr.DeleteLink(ctx, br); err != nil {
				testing.ContextLog(ctx, "Failed to delete bridge while NewBridge has failed: ", err)
			}
		}
	}()
	if err := r.claimBridge(ctx, br); err != nil {
		return "", err
	}
	return br, nil
}

// ReleaseBridge releases the bridge.
func (r *Router) ReleaseBridge(ctx context.Context, br string) error {
	var firstErr error
	collectFirstErr(ctx, &firstErr, r.ipr.FlushIP(ctx, br))
	collectFirstErr(ctx, &firstErr, r.ipr.SetLinkDown(ctx, br))
	collectFirstErr(ctx, &firstErr, r.ipr.DeleteLink(ctx, br))
	return firstErr
}

// NewVethPair returns a veth pair for tests to use. Note that the caller is responsible to call ReleaseVethPair.
func (r *Router) NewVethPair(ctx context.Context) (_, _ string, retErr error) {
	veth := fmt.Sprintf("%s%d", vethPrefix, r.vethID)
	vethPeer := fmt.Sprintf("%s%d", vethPeerPrefix, r.vethID)
	r.vethID++
	if err := r.ipr.AddLink(ctx, veth, "veth", "peer", "name", vethPeer); err != nil {
		return "", "", err
	}
	defer func() {
		if retErr != nil {
			if err := r.ipr.DeleteLink(ctx, veth); err != nil {
				testing.ContextLogf(ctx, "Failed to delete the veth %s while NewVethPair has failed", veth)
			}
		}
	}()
	if err := r.ipr.SetLinkUp(ctx, veth); err != nil {
		return "", "", err
	}
	if err := r.ipr.SetLinkUp(ctx, vethPeer); err != nil {
		return "", "", err
	}
	return veth, vethPeer, nil
}

// ReleaseVethPair release the veth pair.
// Note that each side of the pair can be passed to this method, but the test should only call the method once for each pair.
func (r *Router) ReleaseVethPair(ctx context.Context, veth string) error {
	// If it is a peer side veth name, change it to another side.
	if strings.HasPrefix(veth, vethPeerPrefix) {
		veth = vethPrefix + veth[len(vethPeerPrefix):]
	}
	vethPeer := vethPeerPrefix + veth[len(vethPrefix):]

	var firstErr error
	collectFirstErr(ctx, &firstErr, r.ipr.FlushIP(ctx, veth))
	collectFirstErr(ctx, &firstErr, r.ipr.SetLinkDown(ctx, veth))
	collectFirstErr(ctx, &firstErr, r.ipr.FlushIP(ctx, vethPeer))
	collectFirstErr(ctx, &firstErr, r.ipr.SetLinkDown(ctx, vethPeer))
	// Note that we only need to delete one side.
	collectFirstErr(ctx, &firstErr, r.ipr.DeleteLink(ctx, veth))
	return firstErr
}

// BindVethToBridge binds the veth to bridge.
func (r *Router) BindVethToBridge(ctx context.Context, veth, br string) error {
	return r.ipr.SetBridge(ctx, veth, br)
}

// UnbindVeth unbinds the veth to any other interface.
func (r *Router) UnbindVeth(ctx context.Context, veth string) error {
	return r.ipr.UnsetBridge(ctx, veth)
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

// waitBridgeState polls for the bridge's link status.
func (r *Router) waitBridgeState(ctx context.Context, br string, expectedState ip.LinkState) error {
	const (
		poweredTimeout  = time.Second * 5
		poweredInterval = time.Millisecond * 100
	)
	return testing.Poll(ctx, func(ctx context.Context) error {
		state, err := r.ipr.State(ctx, br)
		if err != nil {
			testing.PollBreak(err)
		}
		if state == expectedState {
			return nil
		}
		return errors.Errorf("unexpected state of bridge %s: got %s, want %s", br, state, expectedState)
	}, &testing.PollOptions{
		Timeout:  poweredTimeout,
		Interval: poweredInterval,
	})
}

// claimBridge claims the bridge from shill. We are doing this because once shill
// manages a device, it runs dhcpcd on it and would mess up our network environment.
// NOTE: This is only for CrOS-base test AP.
// TODO(b/171683002): Find a better way to make sure that shill has already enabled/disabled the bridge. We poll the
// bridge state with ip command for avoiding parsing dbus-send output. shill-test-script might be an alternative:
// https://source.corp.google.com/chromeos_public/src/third_party/chromiumos-overlay/chromeos-base/shill-test-scripts/shill-test-scripts-9999.ebuild
func (r *Router) claimBridge(ctx context.Context, br string) error {
	// Wait for the bridge to be enabled by shill, that is, managed by shill.
	// After shill enables the bridge, because the bridge has not yet connected to any other interface, the state would be UNKNOWN instead of UP.
	if err := r.waitBridgeState(ctx, br, ip.LinkStateUnknown); err != nil {
		return err
	}

	// Disable the bridge to prevent shill from spawning dhcpcd on it.
	if output, err := r.host.Command("dbus-send", "--system", "--type=method_call", "--print-reply",
		"--dest=org.chromium.flimflam", fmt.Sprintf("/device/%s", br), "org.chromium.flimflam.Device.Disable",
	).Output(ctx); err != nil {
		testing.ContextLogf(ctx, "Failed to disable the bridge %q, stdout=%q", br, string(output))
		return errors.Wrapf(err, "failed to set bridge %s down: %v", br, err)
	}

	// Wait for the bridge to become disable.
	if err := r.waitBridgeState(ctx, br, ip.LinkStateDown); err != nil {
		return err
	}

	return r.ipr.SetLinkUp(ctx, br)
}

// removeDevicesWithPrefix removes the devices whose names start with the given prefix.
func (r *Router) removeDevicesWithPrefix(ctx context.Context, prefix string) error {
	devs, err := r.ipr.LinkWithPrefix(ctx, prefix)
	if err != nil {
		return err
	}
	for _, dev := range devs {
		if err := r.ipr.DeleteLink(ctx, dev); err != nil {
			return err
		}
	}
	return nil
}

// isPhyBusyAny returns if the phyID is occupied by a busy interface of any type.
func (r *Router) isPhyBusyAny(phyID int) bool {
	for _, nd := range r.busyIfaces {
		if nd.PhyNum == phyID {
			return true
		}
	}
	return false
}

// isPhyBusy returns if the phyID is occupied by a busy interface of type t.
func (r *Router) isPhyBusy(phyID int, t iw.IfType) bool {
	for _, nd := range r.busyIfaces {
		if nd.PhyNum == phyID && nd.IfType == t {
			return true
		}
	}
	return false
}

// setIfaceBusy marks iface as busy.
func (r *Router) setIfaceBusy(iface string) {
	nd, ok := r.availIfaces[iface]
	if !ok {
		return
	}
	r.busyIfaces[iface] = nd
	delete(r.availIfaces, iface)
}

// freeIface marks iface as free.
func (r *Router) freeIface(iface string) {
	nd, ok := r.busyIfaces[iface]
	if !ok {
		return
	}
	r.availIfaces[iface] = nd
	delete(r.busyIfaces, iface)
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

// MAC returns the MAC address of iface on this router.
func (r *Router) MAC(ctx context.Context, iface string) (net.HardwareAddr, error) {
	return r.ipr.MAC(ctx, iface)
}
