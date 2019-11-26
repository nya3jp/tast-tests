// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	riw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/testing"
)

// Port from Brian's PoC crrev.com/c/1733740

// Router is the object to control an AP router.
type Router struct {
	host   *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	ifaces []*iw.NetDev
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

	r := &Router{host: host}
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
	// TODO: remove this work-around.
	// Dean reported the bug in crbug.com/1030635.
	// Currently we have problems with sshd Signal functionality used to abort daemon.
	// - sshd on router is too old to have it. (7.5 on my test AP, required 7.9 or later)
	// - root login disables privilege separation and Signal funcationality somehow is only allowed with it.
	r.host.Command("killall", "hostapd").Run(ctx)
	r.host.Command("killall", "dnsmasq").Run(ctx)

	var err error
	if err2 := r.removeWifiIfaces(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to remove interfaces, err=%s", err2.Error())
	}
	if err2 := r.host.Close(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to disconnect from router, err=%s", err2.Error())
	}
	return err
}

// SelectInterface traverses all interface and find a suitable interface for the given hostap config.
// TODO: the loop dependency between Router/HostAP* doesn't seem so good.
func (r *Router) SelectInterface(ctx context.Context, conf *HostAPConfig) (string, error) {
	freq, err := channelToFrequency(conf.Channel)
	if err != nil {
		return "", errors.Errorf("channel %d not available", conf.Channel)
	}
	iwr := riw.NewRunner(r.host)
	for _, iface := range r.ifaces {
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
