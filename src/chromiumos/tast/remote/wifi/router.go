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
	riw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wifi/dhcp"
	"chromiumos/tast/remote/wifi/hostap"
	"chromiumos/tast/remote/wifi/utils"
	"chromiumos/tast/testing"
)

// Port from Brian's PoC crrev.com/c/1733740

const workingDir = "/tmp/tast-test/"

// Router is the object to control an AP router.
type Router struct {
	host       *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	ifaces     []*iw.NetDev
	ipIdxMap   [256]int
	busyIfaces map[string]struct{}
}

// HostAPHandle is the handle object of the HostAP service managed under Router object.
type HostAPHandle struct {
	hostapd *hostap.Server
	dhcpd   *dhcp.Server
}

// ServerIP returns the IP of router in the subnet of WiFi.
func (s *HostAPHandle) ServerIP() net.IP {
	return s.dhcpd.ServerIP()
}

// NewRouter connect to the router by SSH and create a Router object.
func NewRouter(ctx context.Context, hostname, keyFile, keyDir string) (*Router, error) {
	sopt := host.SSHOptions{}
	if err := host.ParseSSHTarget(hostname, &sopt); err != nil {
		return nil, errors.Wrap(err, "failed to parse hostname")
	}
	sopt.KeyFile = keyFile
	sopt.KeyDir = keyDir

	host, err := host.NewSSH(ctx, &sopt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create router host")
	}

	r := &Router{
		host:       host,
		busyIfaces: make(map[string]struct{}),
	}
	if err := r.initialize(ctx); err != nil {
		r.Close(ctx)
		return nil, err
	}

	return r, nil
}

func (r *Router) removeWifiIfaces(ctx context.Context) error {
	iwr := riw.NewRunner(r.host)
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

	if err := r.removeWifiIfaces(ctx); err != nil {
		return err
	}

	iwr := riw.NewRunner(r.host)
	wiphys, err := iwr.ListPhys(ctx)
	if err != nil {
		return err
	}
	for i, p := range wiphys {
		w := fmt.Sprintf("managed%d", i)
		testing.ContextLogf(ctx, "Creating wdev %s on wiphy %s", w, p.Name)
		cmd := r.host.Command("iw", "phy", p.Name, "interface", "add", w, "type", "managed")
		if out, err := cmd.Output(ctx); err != nil {
			return errors.Wrapf(err, "failed to create wdev %s on wiphy %s: %s", w, p.Name, string(out))
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
	if err2 := r.host.Close(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to disconnect from router, err=%s", err2.Error())
	}
	return err
}

// SelectInterface traverses all interfaces and find a suitable interface for the given hostap config.
func (r *Router) SelectInterface(ctx context.Context, conf *hostap.Config) (string, error) {
	freq, err := utils.ChannelToFrequency(conf.Channel)
	if err != nil {
		return "", errors.Errorf("channel %d not available", conf.Channel)
	}
	iwr := riw.NewRunner(r.host)
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
func (r *Router) StopHostAP(ctx context.Context, s *HostAPHandle) error {
	var err error
	if err2 := s.hostapd.Stop(ctx); err2 != nil {
		err = errors.Wrap(err2, "failed to stop hostap")
	}
	if err2 := s.dhcpd.Stop(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to stop dhcp: %s", err2.Error())
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
	hostapd, err := hostap.NewServer(ctx, r.host, iface, r.workDir(), conf)
	if err != nil {
		return nil, err
	}
	return hostapd, nil
}

func (r *Router) stopHostapd(ctx context.Context, hostapd *hostap.Server) error {
	r.setIfaceBusy(hostapd.Interface(), false)
	return hostapd.Stop(ctx)
}

func (r *Router) startDHCPd(ctx context.Context, iface string) (*dhcp.Server, error) {
	idx, err := r.getIPIndex()
	if err != nil {
		return nil, err
	}
	dhcpConf := dhcp.NewConfig(idx)
	dhcpd, err := dhcp.NewServer(ctx, r.host, iface, r.workDir(), dhcpConf)
	if err != nil {
		return nil, err
	}
	return dhcpd, nil
}

func (r *Router) stopDHCPd(ctx context.Context, dhcpd *dhcp.Server) error {
	r.freeIPIndex(dhcpd.IPIndex())
	return dhcpd.Stop(ctx)
}

func (r *Router) getIPIndex() (byte, error) {
	for i, v := range r.ipIdxMap {
		if v != 0 {
			continue
		}
		r.ipIdxMap[i] = 1
		return byte(i), nil
	}
	return 0, errors.New("ip index exhausted")
}

func (r *Router) freeIPIndex(i byte) {
	if int(i) >= len(r.ipIdxMap) {
		return
	}
	r.ipIdxMap[i] = 0
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
