// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package router

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
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
	"chromiumos/tast/remote/wificell/wifiutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// legacyRouterStruct is used to control the legacy wireless router and stores state of the router.
type legacyRouterStruct struct {
	BaseRouterStruct
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

// newLegacyRouter prepares initial test AP state (e.g., initializing wiphy/wdev).
// ctx is the deadline for the step and daemonCtx is the lifetime for background
// daemons.
func newLegacyRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string) (Legacy, error) {
	r := &legacyRouterStruct{
		BaseRouterStruct: BaseRouterStruct{
			host:  host,
			name:  name,
			rtype: LegacyT,
		},
		phys:          make(map[int]*iw.Phy),
		availIfaces:   make(map[string]*iw.NetDev),
		busyIfaces:    make(map[string]*iw.NetDev),
		iwr:           remote_iw.NewRemoteRunner(host),
		ipr:           remote_ip.NewRemoteRunner(host),
		logCollectors: make(map[string]*log.Collector),
	}
	shortCtx, cancel := ReserveForRouterClose(ctx)
	defer cancel()

	ctx, st := timing.Start(shortCtx, "initialize")
	defer st.End()

	board, err := hostBoard(shortCtx, r.host)
	if err != nil {
		r.Close(shortCtx)
		return nil, err
	}
	r.board = board

	// Clean up Autotest working dir, in case we're out of space.
	// NB: we need 'sh' to handle the glob.
	if err := r.host.CommandContext(shortCtx, "sh", "-c", strings.Join([]string{"rm", "-rf", autotestWorkdirGlob}, " ")).Run(); err != nil {
		r.Close(shortCtx)
		return nil, errors.Wrapf(err, "failed to remove workdir %q", autotestWorkdirGlob)
	}

	// Set up working dir.
	if err := r.host.CommandContext(shortCtx, "rm", "-rf", r.workDir()).Run(); err != nil {
		r.Close(shortCtx)
		return nil, errors.Wrapf(err, "failed to remove workdir %q", r.workDir())
	}
	if err := r.host.CommandContext(shortCtx, "mkdir", "-p", r.workDir()).Run(); err != nil {
		r.Close(shortCtx)
		return nil, errors.Wrapf(err, "failed to create workdir %q", r.workDir())
	}

	// Start log collectors with daemonCtx as it should live longer than current
	// stage when we are in precondition.
	if err := r.startLogCollectors(daemonCtx); err != nil {
		r.Close(shortCtx)
		return nil, errors.Wrap(err, "failed to start loggers")
	}

	if err := r.setupWifiPhys(shortCtx); err != nil {
		r.Close(shortCtx)
		return nil, err
	}
	if err := r.removeDevicesWithPrefix(shortCtx, bridgePrefix); err != nil {
		r.Close(shortCtx)
		return nil, err
	}
	// Note that we only need to remove one side of each veth pair.
	if err := r.removeDevicesWithPrefix(shortCtx, vethPrefix); err != nil {
		r.Close(shortCtx)
		return nil, err
	}

	killHostapdDhcp := func() {
		shortCtx, st := timing.Start(shortCtx, "killHostapdDhcp")
		defer st.End()

		// Kill remaining hostapd/dnsmasq.
		hostapd.KillAll(shortCtx, r.host)
		dhcp.KillAll(shortCtx, r.host)
	}
	killHostapdDhcp()

	// TODO(crbug.com/839164): Current CrOS on router haven't got the fix in crrev.com/c/1979112.
	// Let's keep the truncate and remove it after we have router updated.
	const umaEventsPath = "/var/lib/metrics/uma-events"
	if err := r.host.CommandContext(shortCtx, "truncate", "-s", "0", "-c", umaEventsPath).Run(); err != nil {
		// Don't return error here, as it might not bother the test as long as it does not
		// fill the whole partition.
		testing.ContextLogf(shortCtx, "Failed to truncate %s: %v", umaEventsPath, err)
	}

	if err := r.iwr.SetRegulatoryDomain(shortCtx, "US"); err != nil {
		r.Close(shortCtx)
		return nil, errors.Wrap(err, "failed to set regulatory domain to US")
	}

	stopDaemon := func() {
		shortCtx, st := timing.Start(shortCtx, "stopDaemon")
		defer st.End()

		// Stop upstart job wpasupplicant if available. (ignore the error as it might be stopped already)
		r.host.CommandContext(shortCtx, "stop", "wpasupplicant").Run()
		// Stop avahi if available as it just causes unnecessary network traffic.
		r.host.CommandContext(shortCtx, "stop", "avahi").Run()
	}
	stopDaemon()

	// Configure hw_random, see function doc for more details.
	if err := r.configureRNG(shortCtx); err != nil {
		r.Close(shortCtx)
		return nil, errors.Wrap(err, "failed to configure hw_random")
	}

	return r, nil
}

// GetRouterType returns the router's type
func (r *legacyRouterStruct) GetRouterType() Type {
	return r.rtype
}

// removeWifiIface removes iface with iw command.
func (r *legacyRouterStruct) removeWifiIface(ctx context.Context, iface string) error {
	testing.ContextLogf(ctx, "Deleting wdev %s on %s", iface, r.name)
	return r.iwr.RemoveInterface(ctx, iface)
}

// removeWifiIfaces removes all WiFi interfaces.
func (r *legacyRouterStruct) removeWifiIfaces(ctx context.Context) error {
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
func (r *legacyRouterStruct) setupWifiPhys(ctx context.Context) error {
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
		phyIDBytes, err := r.host.CommandContext(ctx, "cat", fmt.Sprintf("/sys/class/ieee80211/%s/index", p.Name)).Output()
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
func (r *legacyRouterStruct) configureRNG(ctx context.Context) error {
	const rngAvailPath = "/sys/class/misc/hw_random/rng_available"
	const rngCurrentPath = "/sys/class/misc/hw_random/rng_current"
	const wantRng = "tpm-rng"

	out, err := r.host.CommandContext(ctx, "cat", rngCurrentPath).Output()
	if err != nil {
		// The system might not support hw_random, skip the configuration.
		return nil
	}
	current := strings.TrimSpace(string(out))
	if current == wantRng {
		return nil
	}

	out, err = r.host.CommandContext(ctx, "cat", rngAvailPath).Output()
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
	if err := linuxssh.WriteFile(ctx, r.host, rngCurrentPath, []byte(wantRng), 0644); err != nil {
		return err
	}
	return nil
}

// Close cleans the resource used by Router.
func (r *legacyRouterStruct) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "router.Close")
	defer st.End()

	var firstErr error

	// Remove the interfaces that we created.
	for _, nd := range r.availIfaces {
		if err := r.removeWifiIface(ctx, nd.IfName); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove interfaces"))
		}
	}
	for _, nd := range r.busyIfaces {
		testing.ContextLogf(ctx, "iface %s not yet freed", nd.IfName)
		if err := r.removeWifiIface(ctx, nd.IfName); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove interfaces"))
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
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to collect logs"))
	}
	if err := r.stopLogCollectors(ctx); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop loggers"))
	}
	if err := r.host.CommandContext(ctx, "rm", "-rf", r.workDir()).Run(); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove working dir"))
	}
	return firstErr
}

// phy finds an suitable phy for the given channel and target interface type t.
// The selected phy index is returned.
func (r *legacyRouterStruct) phy(ctx context.Context, channel int, t iw.IfType) (int, error) {
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
func (r *legacyRouterStruct) netDev(ctx context.Context, channel int, t iw.IfType) (*iw.NetDev, error) {
	ctx, st := timing.Start(ctx, "netDev")
	defer st.End()

	phyID, err := r.phy(ctx, channel, t)
	if err != nil {
		return nil, err
	}
	return r.netDevWithPhyID(ctx, phyID, t)
}

// netDevWithPhyID finds an available interface on phy#phyID and with given type.
func (r *legacyRouterStruct) netDevWithPhyID(ctx context.Context, phyID int, t iw.IfType) (*iw.NetDev, error) {
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
func (r *legacyRouterStruct) monitorOnInterface(ctx context.Context, iface string) (*iw.NetDev, error) {
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
func (r *legacyRouterStruct) StartHostapd(ctx context.Context, name string, conf *hostapd.Config) (_ *hostapd.Server, retErr error) {
	ctx, st := timing.Start(ctx, "router.StartHostapd")
	defer st.End()

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

func (r *legacyRouterStruct) startHostapdOnIface(ctx context.Context, iface, name string, conf *hostapd.Config) (_ *hostapd.Server, retErr error) {
	ctx, st := timing.Start(ctx, "router.startHostapdOnIface")
	defer st.End()

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
func (r *legacyRouterStruct) StopHostapd(ctx context.Context, hs *hostapd.Server) error {
	var firstErr error
	iface := hs.Interface()
	if err := hs.Close(ctx); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop hostapd"))
	}
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.SetLinkDown(ctx, iface))
	r.freeIface(iface)
	return firstErr
}

// ReconfigureHostapd restarts the hostapd server with the new config. It preserves the interface and the name of the old hostapd server.
func (r *legacyRouterStruct) ReconfigureHostapd(ctx context.Context, hs *hostapd.Server, conf *hostapd.Config) (_ *hostapd.Server, retErr error) {
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
func (r *legacyRouterStruct) StartDHCP(ctx context.Context, name, iface string, ipStart, ipEnd, serverIP, broadcastIP net.IP, mask net.IPMask) (_ *dhcp.Server, retErr error) {
	ctx, st := timing.Start(ctx, "router.StartDHCP")
	defer st.End()

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
func (r *legacyRouterStruct) StopDHCP(ctx context.Context, ds *dhcp.Server) error {
	var firstErr error
	iface := ds.Interface()
	if err := ds.Close(ctx); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop dhcpd"))
	}
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.FlushIP(ctx, iface))
	return firstErr
}

// StartCapture starts a packet capturer.
// After getting a Capturer instance, c, the caller should call r.StopCapture(ctx, c) at the end,
// and use the shortened ctx (provided by r.ReserveForStopCapture(ctx, c)) before r.StopCapture()
// to reserve time for it to run.
func (r *legacyRouterStruct) StartCapture(ctx context.Context, name string, ch int, freqOps []iw.SetFreqOption, pcapOps ...pcap.Option) (ret *pcap.Capturer, retErr error) {
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
func (r *legacyRouterStruct) ReserveForStopCapture(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc) {
	return capturer.ReserveForClose(ctx)
}

// StopCapture stops the packet capturer and releases related resources.
func (r *legacyRouterStruct) StopCapture(ctx context.Context, capturer *pcap.Capturer) error {
	ctx, st := timing.Start(ctx, "router.StopCapture")
	defer st.End()

	var firstErr error
	iface := capturer.Interface()
	if err := capturer.Close(ctx); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop capturer"))
	}
	if err := r.ipr.SetLinkDown(ctx, iface); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, err)
	}
	r.freeIface(iface)
	return firstErr
}

// StartRawCapturer starts a capturer on an existing interface on the router instead of a
// monitor type interface.
// This function is useful for the tests that don't care the 802.11 frames but the behavior
// of upper layer traffic and tests can capture packets directly on AP's interface.
func (r *legacyRouterStruct) StartRawCapturer(ctx context.Context, name, iface string, ops ...pcap.Option) (*pcap.Capturer, error) {
	return pcap.StartCapturer(ctx, r.host, name, iface, r.workDir(), ops...)
}

// ReserveForStopRawCapturer returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.StopRawCapture to reserve time for it.
func (r *legacyRouterStruct) ReserveForStopRawCapturer(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc) {
	return capturer.ReserveForClose(ctx)
}

// StopRawCapturer stops the packet capturer (no extra resources to release).
func (r *legacyRouterStruct) StopRawCapturer(ctx context.Context, capturer *pcap.Capturer) error {
	return capturer.Close(ctx)
}

// NewFrameSender creates a frame sender object.
func (r *legacyRouterStruct) NewFrameSender(ctx context.Context, iface string) (ret *framesender.Sender, retErr error) {
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
func (r *legacyRouterStruct) ReserveForCloseFrameSender(ctx context.Context) (context.Context, context.CancelFunc) {
	// FrameSender don't need close, but we still need some time for freeing interface.
	return ctxutil.Shorten(ctx, 2*time.Second)
}

// CloseFrameSender closes frame sender and releases related resources.
func (r *legacyRouterStruct) CloseFrameSender(ctx context.Context, s *framesender.Sender) error {
	err := r.ipr.SetLinkDown(ctx, s.Interface())
	r.freeIface(s.Interface())
	return err
}

// workDir returns the directory to place temporary files on router.
func (r *legacyRouterStruct) workDir() string {
	return workingDir
}

// NewBridge returns a bridge name for tests to use. Note that the caller is responsible to call ReleaseBridge.
func (r *legacyRouterStruct) NewBridge(ctx context.Context) (_ string, retErr error) {
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
func (r *legacyRouterStruct) ReleaseBridge(ctx context.Context, br string) error {
	var firstErr error
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.FlushIP(ctx, br))
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.SetLinkDown(ctx, br))
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.DeleteLink(ctx, br))
	return firstErr
}

// NewVethPair returns a veth pair for tests to use. Note that the caller is responsible to call ReleaseVethPair.
func (r *legacyRouterStruct) NewVethPair(ctx context.Context) (_, _ string, retErr error) {
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
func (r *legacyRouterStruct) ReleaseVethPair(ctx context.Context, veth string) error {
	// If it is a peer side veth name, change it to another side.
	if strings.HasPrefix(veth, vethPeerPrefix) {
		veth = vethPrefix + veth[len(vethPeerPrefix):]
	}
	vethPeer := vethPeerPrefix + veth[len(vethPrefix):]

	var firstErr error
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.FlushIP(ctx, veth))
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.SetLinkDown(ctx, veth))
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.FlushIP(ctx, vethPeer))
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.SetLinkDown(ctx, vethPeer))
	// Note that we only need to delete one side.
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.DeleteLink(ctx, veth))
	return firstErr
}

// BindVethToBridge binds the veth to bridge.
func (r *legacyRouterStruct) BindVethToBridge(ctx context.Context, veth, br string) error {
	return r.ipr.SetBridge(ctx, veth, br)
}

// UnbindVeth unbinds the veth to any other interface.
func (r *legacyRouterStruct) UnbindVeth(ctx context.Context, veth string) error {
	return r.ipr.UnsetBridge(ctx, veth)
}

// Utilities for resource control.

// uniqueIfaceName returns an unique name for interface with type t.
func (r *legacyRouterStruct) uniqueIfaceName(t iw.IfType) string {
	name := fmt.Sprintf("%s%d", string(t), r.ifaceID)
	r.ifaceID++
	return name
}

// createWifiIface creates an interface on phy with type=t and returns the name of created interface.
func (r *legacyRouterStruct) createWifiIface(ctx context.Context, phyID int, t iw.IfType) (*iw.NetDev, error) {
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
func (r *legacyRouterStruct) waitBridgeState(ctx context.Context, br string, expectedState ip.LinkState) error {
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

// devicePowered polls for the properties and returns the Powered property value of the given device.
// TODO(b/171683002): Find a better way to make sure that shill has already registered and enabled/disabled the device.
func (r *legacyRouterStruct) devicePowered(ctx context.Context, dev string) (bool, error) {
	const (
		poweredTimeout  = time.Second * 5
		poweredInterval = time.Millisecond * 100
	)

	var b []byte
	// The dbus call may fail if shill has not yet noticed and registered the device.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		b, err = r.host.CommandContext(ctx, "gdbus", "call", "--system",
			"--dest", "org.chromium.flimflam",
			"--object-path", fmt.Sprintf("/device/%s", dev),
			"--method", "org.chromium.flimflam.Device.GetProperties",
		).Output()
		return err
	}, &testing.PollOptions{
		Timeout:  poweredTimeout,
		Interval: poweredInterval,
	}); err != nil {
		return false, errors.Wrapf(err, "failed to get the properties of device %s, output=%v", dev, string(b))
	}

	poweredRE := regexp.MustCompile(`'Powered': <(false|true)>`)
	m := poweredRE.FindStringSubmatch(string(b))
	if len(m) != 2 {
		return false, errors.Errorf("failed to parse gdbus output, matches=%v, raw output=%v", m, string(b))
	}
	switch m[1] {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errors.Errorf("unexpected matched string, got %s, want true or false", m[1])
	}
}

// claimBridge claims the bridge from shill. We are doing this because once shill
// manages a device, it runs dhcpcd on it and would mess up our network environment.
// NOTE: This is only for CrOS-base test AP.
// TODO(b/171683002): Find a better way to make sure that shill has already enabled/disabled the bridge. We poll the
// bridge state with ip command for avoiding parsing dbus-send output. shill-test-script might be an alternative:
// https://source.corp.google.com/chromeos_public/src/third_party/chromiumos-overlay/chromeos-base/shill-test-scripts/shill-test-scripts-9999.ebuild
func (r *legacyRouterStruct) claimBridge(ctx context.Context, br string) error {
	p, err := r.devicePowered(ctx, br)
	if err != nil {
		return errors.Wrapf(err, "failed to get the Powered property value of bridge %s", br)
	}

	if p {
		// After shill enables the bridge, because the bridge has not yet connected to any other
		// interface, the state would be UNKNOWN instead of UP.
		// We watch the event "bridge state changes from UNKNOWN to DOWN" later for making sure that
		// the Disable method works successfully, so first make sure the state is already in UNKNOWN.
		if err := r.waitBridgeState(ctx, br, ip.LinkStateUnknown); err != nil {
			return err
		}

		// Disable the bridge to prevent shill from spawning dhcpcd on it.
		if err := r.host.CommandContext(ctx, "dbus-send", "--system", "--type=method_call", "--print-reply",
			"--dest=org.chromium.flimflam", fmt.Sprintf("/device/%s", br), "org.chromium.flimflam.Device.Disable",
		).Run(ssh.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to set bridge %s down", br)
		}

		// Wait for the bridge to become disable.
		if err := r.waitBridgeState(ctx, br, ip.LinkStateDown); err != nil {
			return err
		}
	}

	return r.ipr.SetLinkUp(ctx, br)
}

// removeDevicesWithPrefix removes the devices whose names start with the given prefix.
func (r *legacyRouterStruct) removeDevicesWithPrefix(ctx context.Context, prefix string) error {
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
func (r *legacyRouterStruct) isPhyBusyAny(phyID int) bool {
	for _, nd := range r.busyIfaces {
		if nd.PhyNum == phyID {
			return true
		}
	}
	return false
}

// isPhyBusy returns if the phyID is occupied by a busy interface of type t.
func (r *legacyRouterStruct) isPhyBusy(phyID int, t iw.IfType) bool {
	for _, nd := range r.busyIfaces {
		if nd.PhyNum == phyID && nd.IfType == t {
			return true
		}
	}
	return false
}

// setIfaceBusy marks iface as busy.
func (r *legacyRouterStruct) setIfaceBusy(iface string) {
	nd, ok := r.availIfaces[iface]
	if !ok {
		return
	}
	r.busyIfaces[iface] = nd
	delete(r.availIfaces, iface)
}

// freeIface marks iface as free.
func (r *legacyRouterStruct) freeIface(iface string) {
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
func (r *legacyRouterStruct) startLogCollectors(ctx context.Context) error {
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
func (r *legacyRouterStruct) collectLogs(ctx context.Context, suffix string) error {
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
func (r *legacyRouterStruct) stopLogCollectors(ctx context.Context) error {
	var firstErr error
	for _, c := range r.logCollectors {
		if err := c.Close(); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, err)
		}
	}
	return firstErr
}

// cloneMAC clones the MAC address of src to dst.
func (r *legacyRouterStruct) cloneMAC(ctx context.Context, dst, src string) error {
	mac, err := r.ipr.MAC(ctx, src)
	if err != nil {
		return err
	}
	return r.ipr.SetMAC(ctx, dst, mac)
}

// CollectLogs downloads log files from router to OutDir.
func (r *legacyRouterStruct) CollectLogs(ctx context.Context) error {
	return r.collectLogs(ctx, "")
}

// SetAPIfaceDown brings down the interface that the APIface uses.
func (r *legacyRouterStruct) SetAPIfaceDown(ctx context.Context, iface string) error {
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

	cmd := host.CommandContext(ctx, "cat", lsbReleasePath)
	out, err := cmd.Output()
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
func (r *legacyRouterStruct) MAC(ctx context.Context, iface string) (net.HardwareAddr, error) {
	return r.ipr.MAC(ctx, iface)
}
