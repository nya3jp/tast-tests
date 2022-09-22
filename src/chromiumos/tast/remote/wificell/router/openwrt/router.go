// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package openwrt

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/network/ip"
	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/common/utils"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	remoteIp "chromiumos/tast/remote/network/ip"
	remoteIw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/http"
	"chromiumos/tast/remote/wificell/log"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/remote/wificell/router/common"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// Router controls an OpenWrt router and stores the router state.
type Router struct {
	host             *ssh.Conn
	name             string
	routerType       support.RouterType
	syslogdCollector *log.SyslogdCollector
	iwr              *remoteIw.Runner
	ipr              *remoteIp.Runner
	phys             map[int]*iw.Phy // map from phy idx to iw.Phy.
	im               *common.IfaceManager
	activeServices   activeServices
	nextBridgeID     int
	nextVethID       int
	closed           bool
	workDirPath      string
}

// activeServices keeps a record of what services have been started and not yet
// stopped manually so that they can be stopped during Router.Close.
type activeServices struct {
	hostapd    []*hostapd.Server
	dhcp       []*dhcp.Server
	capture    []*pcap.Capturer
	rawCapture []*pcap.Capturer
}

// NewRouter prepares initial test AP state (e.g., initializing wiphy/wdev).
// ctx is the deadline for the step and daemonCtx is the lifetime for background
// daemons.
func NewRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string) (*Router, error) {
	testing.ContextLogf(ctx, "Creating new OpenWrt router controller for router %q", name)
	r := &Router{
		host:           host,
		name:           name,
		routerType:     support.OpenWrtT,
		iwr:            remoteIw.NewRemoteRunner(host),
		ipr:            remoteIp.NewRemoteRunner(host),
		phys:           make(map[int]*iw.Phy),
		activeServices: activeServices{},
		closed:         false,
		workDirPath:    common.BuildWorkingDirPath(),
	}
	r.im = common.NewRouterIfaceManager(r, r.iwr)

	shortCtx, cancel := ctxutil.Shorten(ctx, common.RouterCloseContextDuration)
	defer cancel()

	ctx, st := timing.Start(shortCtx, "initialize")
	defer st.End()

	closeBeforeErrorReturn := func(cause error) {
		if err := r.Close(shortCtx); err != nil {
			testing.ContextLogf(shortCtx, "Failed to close after initialization error %v due to %v", cause, err)
		}
	}

	// Start collecting system logs and save logs already in the buffer to a file.
	// The daemonCtx is used for the log collector as it should live longer than
	// the current stage when we are in precondition.
	var err error
	if r.syslogdCollector, err = log.StartSyslogdCollector(daemonCtx, host); err != nil {
		err = errors.Wrap(err, "failed to start syslogd log collector")
		closeBeforeErrorReturn(err)
		return nil, err
	}
	if err := common.CollectSyslogdLogs(daemonCtx, r, r.syslogdCollector, "pre_setup"); err != nil {
		err = errors.Wrap(err, "failed to collect syslogd logs before setup actions")
		closeBeforeErrorReturn(err)
		return nil, err
	}

	// Set up working dir.
	if err := r.host.CommandContext(shortCtx, "rm", "-rf", r.workDir()).Run(); err != nil {
		err = errors.Wrapf(err, "failed to remove workdir %q", r.workDir())
		closeBeforeErrorReturn(err)
		return nil, err
	}
	if err := r.host.CommandContext(shortCtx, "mkdir", "-p", r.workDir()).Run(); err != nil {
		err = errors.Wrapf(err, "failed to create workdir %q", r.workDir())
		closeBeforeErrorReturn(err)
		return nil, err
	}

	if err := r.killHostapdDHCP(shortCtx); err != nil {
		err = errors.Wrap(err, "failed to kill hostapd and DHCP")
		closeBeforeErrorReturn(err)
		return nil, err
	}

	if err := r.setupWifiPhys(shortCtx); err != nil {
		closeBeforeErrorReturn(err)
		return nil, err
	}

	// Clean up any lingering bridge and veth ifaces.
	if err := common.RemoveAllBridgeIfaces(ctx, r.ipr); err != nil {
		closeBeforeErrorReturn(err)
		return nil, err
	}
	if err := common.RemoveAllVethIfaces(ctx, r.ipr); err != nil {
		closeBeforeErrorReturn(err)
		return nil, err
	}

	// Save logs collected from setup actions.
	if err := common.CollectSyslogdLogs(daemonCtx, r, r.syslogdCollector, "post_setup"); err != nil {
		err = errors.Wrap(err, "failed to collect syslogd logs after setup actions")
		closeBeforeErrorReturn(err)
		return nil, err
	}

	testing.ContextLogf(ctx, "Created new OpenWrt router controller for router %q", r.name)
	return r, nil
}

// Close cleans the resource used by Router.
func (r *Router) Close(ctx context.Context) error {
	if r.closed {
		return errors.Errorf("router controller for router %s router %q already closed", r.RouterType().String(), r.RouterName())
	}
	ctx, st := timing.Start(ctx, "router.Close")
	defer st.End()

	testing.ContextLogf(ctx, "Closing OpenWrt router controller for router %q", r.name)

	var firstErr error

	// Collect closing log to facilitate debugging.
	if err := common.CollectSyslogdLogs(ctx, r, r.syslogdCollector, "pre_close"); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to collect syslogd logs before close actions"))
	}

	// Remove the interfaces that we created.
	for _, nd := range r.im.Available {
		if err := r.im.Remove(ctx, nd.IfName); err != nil {
			utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove interfaces"))
		}
	}
	for _, nd := range r.im.Busy {
		testing.ContextLogf(ctx, "iface %s not yet freed", nd.IfName)
		if err := r.im.Remove(ctx, nd.IfName); err != nil {
			utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove interfaces"))
		}
	}

	// Stop any services that were not manually stopped already.
	if err := r.closeActiveServices(ctx); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop still active services"))
	}

	// Clean up any lingering bridge and veth ifaces.
	if err := common.RemoveAllBridgeIfaces(ctx, r.ipr); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, err)
	}
	if err := common.RemoveAllVethIfaces(ctx, r.ipr); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, err)
	}

	// Clean working dir.
	if err := r.host.CommandContext(ctx, "rm", "-rf", r.workDir()).Run(); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove working dir"))
	}

	// Collect closing log to facilitate debugging.
	if err := common.CollectSyslogdLogs(ctx, r, r.syslogdCollector, "post_close"); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to collect syslogd logs after close actions"))
	}
	if err := r.syslogdCollector.Close(); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop syslogd log collector"))
	}

	testing.ContextLogf(ctx, "Closed OpenWrt router controller for router %q", r.name)
	r.closed = true
	return firstErr
}

func (r *Router) closeActiveServices(ctx context.Context) error {
	var firstError error
	for len(r.activeServices.hostapd) != 0 {
		if err := r.StopHostapd(ctx, r.activeServices.hostapd[0]); err != nil {
			utils.CollectFirstErr(ctx, &firstError, err)
		}
	}
	for len(r.activeServices.dhcp) != 0 {
		if err := r.StopDHCP(ctx, r.activeServices.dhcp[0]); err != nil {
			utils.CollectFirstErr(ctx, &firstError, err)
		}
	}
	for len(r.activeServices.capture) != 0 {
		if err := r.StopCapture(ctx, r.activeServices.capture[0]); err != nil {
			utils.CollectFirstErr(ctx, &firstError, err)
		}
	}
	for len(r.activeServices.rawCapture) != 0 {
		if err := r.StopRawCapturer(ctx, r.activeServices.rawCapture[0]); err != nil {
			utils.CollectFirstErr(ctx, &firstError, err)
		}
	}
	return firstError
}

// RouterType returns the router type.
func (r *Router) RouterType() support.RouterType {
	return r.routerType
}

// RouterName returns the name of the managed router device.
func (r *Router) RouterName() string {
	return r.name
}

// StartReboot initiates a reboot of the router host.
//
// Close must be called prior to StartReboot, not after.
//
// This Router instance will be unable to interact with the host after calling
// this, as the connection to the host will be severed. To use this host
// again, create a new Router instance with a new connection after the host is
// fully rebooted.
func (r *Router) StartReboot(ctx context.Context) error {
	_ = r.host.CommandContext(ctx, "reboot").Run()
	return nil
}

// workDir returns the directory to place temporary files on router.
func (r *Router) workDir() string {
	return r.workDirPath
}

// CollectLogs dumps collected syslogd logs to a file.
func (r *Router) CollectLogs(ctx context.Context) error {
	return common.CollectSyslogdLogs(ctx, r, r.syslogdCollector, "")
}

// killHostapdDHCP forcibly kills any hostapd and dhcp processes.
func (r *Router) killHostapdDHCP(ctx context.Context) error {
	shortCtx, st := timing.Start(ctx, "killHostapdDHCP")
	defer st.End()
	var firstErr error
	if err := r.host.CommandContext(shortCtx, "/etc/init.d/dnsmasq", "stop").Run(); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop dnsmasq service which manages core OpenWrt DHCP servers"))
	}
	if err := r.host.CommandContext(shortCtx, "/etc/init.d/dnsmasq", "disable").Run(); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to disable dnsmasq service which manages core OpenWrt DHCP servers"))
	}
	if err := r.host.CommandContext(shortCtx, "/etc/init.d/wpad", "stop").Run(); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop wpad service which manages core OpenWrt hostapd processes"))
	}
	if err := r.host.CommandContext(shortCtx, "/etc/init.d/wpad", "disable").Run(); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to disable wpad service which manages core OpenWrt hostapd processes"))
	}
	if err := hostapd.KillAll(shortCtx, r.host); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to kill all hostapd processes"))
	}
	if err := dhcp.KillAll(shortCtx, r.host); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to kill all dhcp processes"))
	}
	return nil
}

// setupWifiPhys fills r.phys and enables their antennas.
func (r *Router) setupWifiPhys(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "setupWifiPhys")
	defer st.End()

	if err := r.im.RemoveAll(ctx); err != nil {
		return err
	}
	phys, _, err := r.iwr.ListPhys(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list phys")
	}
	if len(phys) == 0 {
		return errors.New("Expect at least one wireless phy; found nothing")
	}
	for _, p := range phys {
		// Get phy index using system config and map it.
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

// StartHostapd starts the hostapd server.
func (r *Router) StartHostapd(ctx context.Context, name string, conf *hostapd.Config) (*hostapd.Server, error) {
	ctx, st := timing.Start(ctx, "router.StartHostapd")
	defer st.End()

	if err := conf.SecurityConfig.InstallRouterCredentials(ctx, r.host, r.workDir()); err != nil {
		return nil, errors.Wrap(err, "failed to install router credentials")
	}

	// Fail fast if WEP is configured since it is hard to deduce from logs.
	if isWEP, err := common.HostapdSecurityConfigIsWEP(conf.SecurityConfig); err != nil {
		return nil, errors.Wrap(err, "failed to check if hostapd security config uses wep")
	} else if isWEP {
		return nil, errors.Wrap(err, "WEP security is not supported on OpenWrt routers")
	}

	nd, err := r.netDev(ctx, conf.Channel, iw.IfTypeManaged)
	if err != nil {
		return nil, err
	}
	iface := nd.IfName
	r.im.SetBusy(iface)
	hs, err := r.startHostapdOnIface(ctx, iface, name, conf)
	if err != nil {
		r.im.SetAvailable(iface)
		return nil, err
	}
	r.activeServices.hostapd = append(r.activeServices.hostapd, hs)
	return hs, nil
}

// StopHostapd stops the hostapd server.
func (r *Router) StopHostapd(ctx context.Context, hs *hostapd.Server) error {
	var firstErr error
	iface := hs.Interface()
	if err := hs.Close(ctx); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop hostapd"))
	}
	utils.CollectFirstErr(ctx, &firstErr, r.ipr.SetLinkDown(ctx, iface))
	r.im.SetAvailable(iface)

	// Remove from active services.
	for i, service := range r.activeServices.hostapd {
		if hs == service {
			active := make([]*hostapd.Server, 0)
			active = append(active, r.activeServices.hostapd[:i]...)
			active = append(active, r.activeServices.hostapd[i+1:]...)
			r.activeServices.hostapd = active
			break
		}
	}
	return firstErr
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
	for _, nd := range r.im.Available {
		if nd.PhyNum == phyID && nd.IfType == t {
			return nd, nil
		}
	}
	// No available interface on phy, create one.
	return r.createWifiIface(ctx, phyID, t)
}

// createWifiIface creates an interface on phy with type=t and returns the name of created interface.
func (r *Router) createWifiIface(ctx context.Context, phyID int, t iw.IfType) (*iw.NetDev, error) {
	ctx, st := timing.Start(ctx, "createWifiIface")
	defer st.End()
	phyName := r.phys[phyID].Name
	return r.im.Create(ctx, phyName, phyID, t)
}

// phy finds a suitable phy for the given channel and target interface type t.
// The selected phy index is returned.
func (r *Router) phy(ctx context.Context, channel int, t iw.IfType) (int, error) {
	freq, err := hostapd.ChannelToFrequency(channel)
	if err != nil {
		return 0, errors.Errorf("channel %d not available", channel)
	}
	// Try to find an idle phy which is suitable.
	for id, phy := range r.phys {
		if r.im.IsPhyBusy(id, t) {
			continue
		}
		if phySupportsFrequency(phy, freq) {
			return id, nil
		}
	}
	// Try to find any phy which is suitable, even a busy one.
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

func (r *Router) startHostapdOnIface(ctx context.Context, iface, name string, conf *hostapd.Config) (_ *hostapd.Server, retErr error) {
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

// ReconfigureHostapd restarts the hostapd server with the new config. It
// preserves the interface and the name of the old hostapd server.
func (r *Router) ReconfigureHostapd(ctx context.Context, hs *hostapd.Server, conf *hostapd.Config) (*hostapd.Server, error) {
	iface := hs.Interface()
	name := hs.Name()
	if err := r.StopHostapd(ctx, hs); err != nil {
		return nil, errors.Wrap(err, "failed to stop hostapd server")
	}
	r.im.SetBusy(iface)
	hs, err := r.startHostapdOnIface(ctx, iface, name, conf)
	if err != nil {
		r.im.SetAvailable(iface)
		return nil, err
	}
	r.activeServices.hostapd = append(r.activeServices.hostapd, hs)
	return hs, nil
}

// StartDHCP starts the DHCP server and configures the server IP. If DNS functionality is
// not required, set dnsOpt to nil.
func (r *Router) StartDHCP(ctx context.Context, name, iface string, ipStart, ipEnd, serverIP, broadcastIP net.IP, mask net.IPMask, dnsOpt *dhcp.DNSOption) (_ *dhcp.Server, retErr error) {
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
	ds, err := dhcp.StartServer(ctx, r.host, name, iface, r.workDir(), ipStart, ipEnd, dnsOpt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start DHCP server")
	}
	r.activeServices.dhcp = append(r.activeServices.dhcp, ds)
	return ds, nil
}

// StopDHCP stops the DHCP server and flushes the interface.
func (r *Router) StopDHCP(ctx context.Context, ds *dhcp.Server) error {
	var firstErr error
	iface := ds.Interface()
	if err := ds.Close(ctx); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop dhcpd"))
	}
	utils.CollectFirstErr(ctx, &firstErr, r.ipr.FlushIP(ctx, iface))

	// Remove from active services.
	for i, service := range r.activeServices.dhcp {
		if ds == service {
			active := make([]*dhcp.Server, 0)
			active = append(active, r.activeServices.dhcp[:i]...)
			active = append(active, r.activeServices.dhcp[i+1:]...)
			r.activeServices.dhcp = active
			break
		}
	}
	return firstErr
}

// StartHTTP starts the HTTP server.
// TODO(b/242864063): Test and implement the functionality of HTTP server in openwrt router.
func (r *Router) StartHTTP(ctx context.Context, name, iface, redirectAddr string, port, statusCode int) (_ *http.Server, retErr error) {
	return nil, nil
}

// StopHTTP stops the HTTP server.
// TODO(b/242864063): Test and implement the functionality of HTTP server in openwrt router.
func (r *Router) StopHTTP(ctx context.Context, httpServer *http.Server) error {
	return nil
}

// StartCapture starts a packet capturer.
func (r *Router) StartCapture(ctx context.Context, name string, ch int, freqOps []iw.SetFreqOption, pcapOps ...pcap.Option) (_ *pcap.Capturer, retErr error) {
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
	shared := r.im.IsPhyBusyAny(nd.PhyNum)

	r.im.SetBusy(iface)
	defer func() {
		if retErr != nil {
			r.im.SetAvailable(iface)
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
	r.activeServices.capture = append(r.activeServices.capture, c)
	return c, nil
}

// StopCapture stops the packet capturer and releases related resources.
func (r *Router) StopCapture(ctx context.Context, capturer *pcap.Capturer) error {
	ctx, st := timing.Start(ctx, "router.StopCapture")
	defer st.End()

	var firstErr error
	iface := capturer.Interface()
	if err := capturer.Close(ctx); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop capturer"))
	}
	if err := r.ipr.SetLinkDown(ctx, iface); err != nil {
		utils.CollectFirstErr(ctx, &firstErr, err)
	}
	r.im.SetAvailable(iface)

	// Remove from active services.
	for i, service := range r.activeServices.capture {
		if capturer == service {
			active := make([]*pcap.Capturer, 0)
			active = append(active, r.activeServices.capture[:i]...)
			active = append(active, r.activeServices.capture[i+1:]...)
			r.activeServices.capture = active
			break
		}
	}
	return firstErr
}

// StartRawCapturer starts a capturer on an existing interface on the router
// instead of a monitor type interface.
func (r *Router) StartRawCapturer(ctx context.Context, name, iface string, ops ...pcap.Option) (*pcap.Capturer, error) {
	capturer, err := pcap.StartCapturer(ctx, r.host, name, iface, r.workDir(), ops...)
	if err != nil {
		return nil, err
	}
	r.activeServices.rawCapture = append(r.activeServices.rawCapture, capturer)
	return capturer, nil
}

// StopRawCapturer stops the packet capturer (no extra resources to release).
func (r *Router) StopRawCapturer(ctx context.Context, capturer *pcap.Capturer) (retErr error) {
	if err := capturer.Close(ctx); err != nil {
		retErr = errors.Wrap(err, "failed to stop capturer")
	}
	// Remove from active services.
	for i, service := range r.activeServices.rawCapture {
		if capturer == service {
			active := make([]*pcap.Capturer, 0)
			active = append(active, r.activeServices.rawCapture[:i]...)
			active = append(active, r.activeServices.rawCapture[i+1:]...)
			r.activeServices.rawCapture = active
			break
		}
	}
	return retErr
}

// ReserveForStopCapture returns a shortened ctx with cancel function.
func (r *Router) ReserveForStopCapture(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc) {
	return capturer.ReserveForClose(ctx)
}

// ReserveForStopRawCapturer returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.StopRawCapture to
// reserve time for it.
func (r *Router) ReserveForStopRawCapturer(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc) {
	return capturer.ReserveForClose(ctx)
}

// SetAPIfaceDown brings down the interface that the APIface uses.
func (r *Router) SetAPIfaceDown(ctx context.Context, iface string) error {
	if err := r.ipr.SetLinkDown(ctx, iface); err != nil {
		return errors.Wrapf(err, "failed to set %s down", iface)
	}
	return nil
}

// MAC returns the MAC address of iface on this router.
func (r *Router) MAC(ctx context.Context, iface string) (net.HardwareAddr, error) {
	return r.ipr.MAC(ctx, iface)
}

// NewBridge returns a bridge name for tests to use. Note that the caller is responsible to call ReleaseBridge.
func (r *Router) NewBridge(ctx context.Context) (string, error) {
	bridgeID := r.nextBridgeID
	r.nextBridgeID++
	return common.NewBridge(ctx, r.ipr, bridgeID)
}

// ReleaseBridge releases the bridge.
func (r *Router) ReleaseBridge(ctx context.Context, br string) error {
	return common.ReleaseBridge(ctx, r.ipr, br)
}

// NewVethPair returns a veth pair for tests to use. Note that the caller is responsible to call ReleaseVethPair.
func (r *Router) NewVethPair(ctx context.Context) (string, string, error) {
	vethID := r.nextVethID
	r.nextVethID++
	return common.NewVethPair(ctx, r.ipr, vethID, true)
}

// ReleaseVethPair releases the veth pair.
// Note that each side of the pair can be passed to this method, but the test should only call the method once for each pair.
func (r *Router) ReleaseVethPair(ctx context.Context, veth string) error {
	return common.ReleaseVethPair(ctx, r.ipr, veth, true)
}

// BindVethToBridge binds the veth to bridge.
func (r *Router) BindVethToBridge(ctx context.Context, veth, br string) error {
	return common.BindVethToBridge(ctx, r.ipr, veth, br)
}

// UnbindVeth unbinds the veth to any other interface.
func (r *Router) UnbindVeth(ctx context.Context, veth string) error {
	return common.UnbindVeth(ctx, r.ipr, veth)
}

// HostIsOpenWrtRouter determines whether the remote host is an OpenWrt router.
func HostIsOpenWrtRouter(ctx context.Context, host *ssh.Conn) (bool, error) {
	deviceInfoPath := "/etc/device_info"
	deviceInfoMatchIfOpenWrt := "(?m)^DEVICE_MANUFACTURER='OpenWrt'$"
	matches, err := common.HostFileContentsMatch(ctx, host, deviceInfoPath, deviceInfoMatchIfOpenWrt)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check if remote file %q contents match %q", deviceInfoPath, deviceInfoMatchIfOpenWrt)
	}
	return matches, nil
}

// NewFrameSender creates a new framesender.Sender object.
func (r *Router) NewFrameSender(ctx context.Context, iface string) (ret *framesender.Sender, retErr error) {
	nd, err := r.monitorOnInterface(ctx, iface)
	if err != nil {
		return nil, err
	}
	r.im.SetBusy(nd.IfName)
	defer func() {
		if retErr != nil {
			r.im.SetAvailable(nd.IfName)
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

// CloseFrameSender closes frame sender and releases related resources.
func (r *Router) CloseFrameSender(ctx context.Context, s *framesender.Sender) error {
	err := r.ipr.SetLinkDown(ctx, s.Interface())
	r.im.SetAvailable(s.Interface())
	return err
}

// monitorOnInterface finds an available monitor type interface on the same phy as a
// busy interface with name=iface.
func (r *Router) monitorOnInterface(ctx context.Context, iface string) (*iw.NetDev, error) {
	var ndev *iw.NetDev
	// Find phy ID of iface.
	for name, nd := range r.im.Busy {
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

// cloneMAC clones the MAC address of src to dst.
func (r *Router) cloneMAC(ctx context.Context, dst, src string) error {
	mac, err := r.ipr.MAC(ctx, src)
	if err != nil {
		return err
	}
	return r.ipr.SetMAC(ctx, dst, mac)
}
