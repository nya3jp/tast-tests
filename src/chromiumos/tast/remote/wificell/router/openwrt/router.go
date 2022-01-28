// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	remoteIp "chromiumos/tast/remote/network/ip"
	remoteIw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell/dhcp"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/log"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/remote/wificell/router/common"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/remote/wificell/router/openwrt/uci"
	"chromiumos/tast/remote/wificell/wifiutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// Router controls an OpenWrt router and stores the router state.
type Router struct {
	host           *ssh.Conn
	name           string
	routerType     support.RouterType
	logCollectors  map[string]*log.Collector // map from log path to its collector.
	iwr            *remoteIw.Runner
	ipr            *remoteIp.Runner
	uci            *uci.Runner
	phys           map[int]*iw.Phy // map from phy idx to iw.Phy.
	im             *common.IfaceManager
	activeServices activeServices
}

// activeServices keeps a record of what services have been started and not yet
// stopped manually so that they can be stopped during Router.Close.
type activeServices struct {
	hostapd    []*hostapd.Server
	dhcp       []*dhcp.Server
	capture    []*pcap.Capturer
	rawCapture []*pcap.Capturer
}

var uciConfigsModified = []string{
	uci.ConfigWireless,
	uci.ConfigDhcp,
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
		logCollectors:  make(map[string]*log.Collector),
		iwr:            remoteIw.NewRemoteRunner(host),
		ipr:            remoteIp.NewRemoteRunner(host),
		uci:            uci.NewRemoteRunner(host),
		phys:           make(map[int]*iw.Phy),
		activeServices: activeServices{},
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

	// Start log collectors with daemonCtx as it should live longer than current
	// stage when we are in precondition.
	if err := common.StartLogCollectors(daemonCtx, r.host, r.logCollectors, logsToCollect, false); err != nil {
		err = errors.Wrap(err, "failed to start loggers")
		closeBeforeErrorReturn(err)
		return nil, err
	}

	if err := r.killHostapdDHCP(shortCtx); err != nil {
		err = errors.Wrap(err, "failed to kill hostapd and DHCP")
		closeBeforeErrorReturn(err)
		return nil, err
	}

	if err := uci.ResetConfigs(shortCtx, r.uci, false, uciConfigsModified...); err != nil {
		err = errors.Wrap(err, "failed to reset uci configs")
		closeBeforeErrorReturn(err)
		return nil, err
	}

	if err := r.setupWifiPhys(shortCtx); err != nil {
		closeBeforeErrorReturn(err)
		return nil, err
	}

	// TODO the -brief flag does not seem to be supported
	//if err := common.RemoveDevicesWithPrefix(shortCtx, r.ipr, common.BridgePrefix); err != nil {
	//	closeBeforeErrorReturn(err)
	//	return nil, err
	//}
	//
	//// Note that we only need to remove one side of each veth pair.
	//if err := common.RemoveDevicesWithPrefix(shortCtx, r.ipr, common.VethPrefix); err != nil {
	//	closeBeforeErrorReturn(err)
	//	return nil, err
	//}

	// TODO stop daemon?

	// TODO configureRNG ?

	testing.ContextLogf(ctx, "Created new OpenWrt router controller for router %q", r.name)
	return r, nil
}

// Close cleans the resource used by Router.
func (r *Router) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "router.Close")
	defer st.End()

	testing.ContextLogf(ctx, "Closing OpenWrt router controller for router %q", r.name)

	var firstErr error

	// Remove the interfaces that we created.
	for _, nd := range r.im.Available {
		if err := r.im.Remove(ctx, nd.IfName); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove interfaces"))
		}
	}
	for _, nd := range r.im.Busy {
		testing.ContextLogf(ctx, "iface %s not yet freed", nd.IfName)
		if err := r.im.Remove(ctx, nd.IfName); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove interfaces"))
		}
	}

	//if err := common.RemoveDevicesWithPrefix(ctx, r.ipr, common.BridgePrefix); err != nil {
	//	wifiutil.CollectFirstErr(ctx, &firstErr, err)
	//}
	//// Note that we only need to remove one side of each veth pair.
	//if err := common.RemoveDevicesWithPrefix(ctx, r.ipr, common.VethPrefix); err != nil {
	//	wifiutil.CollectFirstErr(ctx, &firstErr, err)
	//}

	// Stop any services that were not manually stopped already
	if err := r.closeActiveServices(ctx); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop still active services"))
	}

	// Reset modified uci configs back to their previous states
	if err := uci.ResetConfigs(ctx, r.uci, true, uciConfigsModified...); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to restore uci configs from backups"))
	}
	// Reload services to use backed up configs
	if err := uci.ReloadConfigServices(ctx, r.uci, uciConfigsModified...); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to reload configs using backup configs"))
	}

	// Collect closing log to facilitate debugging
	if err := common.CollectLogs(ctx, r, r.logCollectors, logsToCollect, ".close"); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to collect closing logs"))
	}
	if err := common.StopLogCollectors(ctx, r.logCollectors); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop loggers"))
	}

	// Clean working dir
	//if err := r.host.CommandContext(ctx, "rm", "-rf", r.workDir()).Run(); err != nil {
	//	wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to remove working dir"))
	//}

	testing.ContextLogf(ctx, "Closed OpenWrt router controller for router %q", r.name)

	return firstErr
}

func (r *Router) closeActiveServices(ctx context.Context) error {
	for len(r.activeServices.hostapd) != 0 {
		if err := r.StopHostapd(ctx, r.activeServices.hostapd[0]); err != nil {
			return err
		}
	}
	for len(r.activeServices.dhcp) != 0 {
		if err := r.StopDHCP(ctx, r.activeServices.dhcp[0]); err != nil {
			return err
		}
	}
	for len(r.activeServices.capture) != 0 {
		if err := r.StopCapture(ctx, r.activeServices.capture[0]); err != nil {
			return err
		}
	}
	for len(r.activeServices.rawCapture) != 0 {
		if err := r.StopRawCapturer(ctx, r.activeServices.rawCapture[0]); err != nil {
			return err
		}
	}
	return nil
}

// RouterType returns the router type.
func (r *Router) RouterType() support.RouterType {
	return r.routerType
}

// RouterTypeName returns the human-readable name this Router's RouterType
func (r *Router) RouterTypeName() string {
	return "OpenWrt"
}

// RouterName returns the name of the managed router device.
func (r *Router) RouterName() string {
	return r.name
}

// workDir returns the directory to place temporary files on router.
func (r *Router) workDir() string {
	return common.WorkingDir
}

// logsToCollect is the list of files on router to collect.
var logsToCollect = []string{
	"/var/log/messages",
}

// CollectLogs downloads log files from router to OutDir.
func (r *Router) CollectLogs(ctx context.Context) error {
	return common.CollectLogs(ctx, r, r.logCollectors, logsToCollect, "")
}

// killHostapdDHCP forcibly kills any hostapd and dhcp processes
func (r *Router) killHostapdDHCP(ctx context.Context) error {
	shortCtx, st := timing.Start(ctx, "killHostapdDHCP")
	defer st.End()
	// Kill the processes if they exist, ignoring errors that result if they do not
	_ = hostapd.KillAll(shortCtx, r.host)
	_ = dhcp.KillAll(shortCtx, r.host)
	return nil
}

// setupWifiPhys fills r.phys and enables their antennas.
func (r *Router) setupWifiPhys(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "setupWifiPhys")
	defer st.End()

	if err := r.im.RemoveAll(ctx); err != nil {
		return err
	}
	phys, err := r.iwr.ListPhys(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list phys")
	}
	if len(phys) == 0 {
		return errors.New("Expect at least one wireless phy; found nothing")
	}
	for _, p := range phys {
		// Not supported?
		//if p.SupportSetAntennaMask() {
		//	if err := r.iwr.SetAntennaBitmap(ctx, p.Name, p.TxAntenna, p.RxAntenna); err != nil {
		//		return errors.Wrapf(err, "failed to set bitmap for %s with tx=%d and rx=%d", p.Name, p.TxAntenna, p.RxAntenna)
		//	}
		//}

		// Get phy index using system config and map it
		phyIDBytes, err := r.host.CommandContext(ctx, "cat", fmt.Sprintf("/sys/class/ieee80211/%s/index", p.Name)).Output()
		if err != nil {
			return errors.Wrapf(err, "failed to get phy idx for %s", p.Name)
		}
		phyID, err := strconv.Atoi(strings.TrimSpace(string(phyIDBytes)))
		if err != nil {
			return errors.Wrapf(err, "invalid phy idx %s", string(phyIDBytes))
		}
		r.phys[phyID] = p

		uciWifiDeviceSection := r.phyIDToUciWifiDeviceSection(phyID)

		// Enable device
		if err := r.uci.Set(ctx, uci.ConfigWireless, uciWifiDeviceSection, "disabled", "0"); err != nil {
			return errors.Wrapf(err, "failed to set disabled to 0 for %s", p.Name)
		}

		// Set the corresponding UCI wifi-device to use US for the regulatory domain
		if err := r.uci.Set(ctx, uci.ConfigWireless, uciWifiDeviceSection, "country", "US"); err != nil {
			return errors.Wrapf(err, "failed to set regulatory domain to US for %s", p.Name)
		}
	}

	// Save changes to wireless devices
	if err := uci.CommitAndReloadConfig(ctx, r.uci, uci.ConfigWireless); err != nil {
		err = errors.Wrap(err, "failed to update wireless config")
	}
	return nil
}

func (r *Router) phyIDToUciWifiDeviceSection(phyID int) string {
	return fmt.Sprintf("@wifi-device[%d]", phyID)
}

// StartHostapd starts the hostapd server.
func (r *Router) StartHostapd(ctx context.Context, name string, conf *hostapd.Config) (*hostapd.Server, error) {
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
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop hostapd"))
	}
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.SetLinkDown(ctx, iface))
	r.im.SetAvailable(iface)

	// remove from active services
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
	// Try to find an idle phy which is suitable
	for id, phy := range r.phys {
		if r.im.IsPhyBusy(id, t) {
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

// ReconfigureHostapd restarts the hostapd server with the new config. It preserves the interface and the name of the old hostapd server.
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

// StartDHCP starts the DHCP server and configures the server IP.
func (r *Router) StartDHCP(ctx context.Context, name, iface string, ipStart, ipEnd, serverIP, broadcastIP net.IP, mask net.IPMask) (_ *dhcp.Server, retErr error) {
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
	r.activeServices.dhcp = append(r.activeServices.dhcp, ds)
	return ds, nil
}

// StopDHCP stops the DHCP server and flushes the interface.
func (r *Router) StopDHCP(ctx context.Context, ds *dhcp.Server) error {
	var firstErr error
	iface := ds.Interface()
	if err := ds.Close(ctx); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop dhcpd"))
	}
	wifiutil.CollectFirstErr(ctx, &firstErr, r.ipr.FlushIP(ctx, iface))

	// remove from active services
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
	// TODO
	// remove from active services
	for i, service := range r.activeServices.capture {
		if capturer == service {
			active := make([]*pcap.Capturer, 0)
			active = append(active, r.activeServices.capture[:i]...)
			active = append(active, r.activeServices.capture[i+1:]...)
			r.activeServices.capture = active
			break
		}
	}
	return nil
}

// StartRawCapturer starts a capturer on an existing interface on the router instead of a
// monitor type interface.
func (r *Router) StartRawCapturer(ctx context.Context, name, iface string, ops ...pcap.Option) (*pcap.Capturer, error) {
	capturer, err := pcap.StartCapturer(ctx, r.host, name, iface, r.workDir(), ops...)
	if err != nil {
		return nil, err
	}
	r.activeServices.rawCapture = append(r.activeServices.rawCapture, capturer)
	return capturer, nil
}

// StopRawCapturer stops the packet capturer (no extra resources to release).
func (r *Router) StopRawCapturer(ctx context.Context, capturer *pcap.Capturer) error {
	if err := capturer.Close(ctx); err != nil {
		return err
	}
	// remove from active services
	for i, service := range r.activeServices.rawCapture {
		if capturer == service {
			active := make([]*pcap.Capturer, 0)
			active = append(active, r.activeServices.rawCapture[:i]...)
			active = append(active, r.activeServices.rawCapture[i+1:]...)
			r.activeServices.rawCapture = active
			break
		}
	}
	return nil
}

// ReserveForStopCapture returns a shortened ctx with cancel function.
func (r *Router) ReserveForStopCapture(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc) {
	return capturer.ReserveForClose(ctx)
}

// ReserveForStopRawCapturer returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.StopRawCapture to reserve time for it.
func (r *Router) ReserveForStopRawCapturer(ctx context.Context, capturer *pcap.Capturer) (context.Context, context.CancelFunc) {
	return capturer.ReserveForClose(ctx)
}
