// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"math/rand"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
)

// Port from Brian's PoC crrev.com/c/1733740

// Router is the object to control an AP router.
type Router struct {
	host   *dut.DUT // TODO(crbug.com/1019537): use a more suitable ssh object instead of DUT. DUT is specific for Chromebook.
	ifaces []*iw.NetDev
	phys   map[string]*iw.Phy
}

// NewRouter connect to the router by SSH and create a Router object.
// TODO: do we always use key auth?
func NewRouter(ctx context.Context, hostname, keyFile, keyDir string) (*Router, error) {
	host, err := dut.New(hostname, keyFile, keyDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create router host")
	}

	if err := host.Connect(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to connect to router")
	}

	r := &Router{host: host}
	if err := r.initialize(ctx); err != nil {
		r.Close(ctx)
		return nil, err
	}

	return r, nil
}

// initialize prepares initial test AP state (e.g., initializing wiphy/wdev).
func (r *Router) initialize(ctx context.Context) error {
	iwr := iw.NewRunnerWithDUT(r.host)

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
	// Keep a cache of the interface/phy list on r.
	r.ifaces = ifaces
	r.phys = make(map[string]*iw.Phy)
	for _, p := range wiphys {
		r.phys[p.Name] = p
	}
	return err
}

// Close disconnects the SSH to router.
func (r *Router) Close(ctx context.Context) error {
	// TODO: remove this work-around.
	// Currently we have problems with sshd Signal functionality used to abort daemon.
	// - sshd on router is too old to have it. (7.5 on my test AP, required 7.9 or later)
	// - root login disables privilege separation and Signal funcationality somehow is only allowed with it.
	r.host.Command("killall", "hostapd").Run(ctx)
	r.host.Command("killall", "dnsmasq").Run(ctx)

	return r.host.Disconnect(ctx)
}

// SelectInterface traverses all interface and find a suitable interface for the given hostap config.
// TODO: the loop dependency between Router/HostAP* doesn't seem so good.
func (r *Router) SelectInterface(conf *HostAPConfig) (string, error) {
	freq, err := channelToFrequency(conf.Channel)
	if err != nil {
		return "", errors.Errorf("channel %d not available", conf.Channel)
	}
	for _, iface := range r.ifaces {
		// TODO: this is awkward, we should normalize the id of phy in iw_runner.
		// either all in string name or int num.
		phyName := fmt.Sprintf("phy%d", iface.PhyNum)
		phy, ok := r.phys[phyName]
		if !ok {
			return "", errors.Errorf("interface %s has unknown phy number=%d", iface.IfName, iface.PhyNum)
		}

		// TODO: more checks.
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

// GetAPWdev returns the interface name of the Nth iw.NetDev configured on this AP.
func (r *Router) GetAPWdev(idx int) (string, error) {
	if idx >= len(r.ifaces) {
		return "", errors.Errorf("not enough wdevs available (%d >= %d)", idx, len(r.ifaces))
	}
	return r.ifaces[idx].IfName, nil
}

// RandomSSID returns a random SSID of length 30 and given prefix.
func RandomSSID(prefix string) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// Generate 30-char SSID, including prefix
	n := 30 - len(prefix)
	s := make([]byte, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return prefix + string(s)
}
