// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type wDev struct {
	name  string
	wiphy string
}

type Router struct {
	host   *dut.DUT
	ifaces []wDev
}

func NewRouter(ctx context.Context, host string) (*Router, error) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		return nil, errors.New("Failed to get DUT")
	}

	testing.ContextLog(ctx, "Connecting to host ", host)
	d, err := dut.New(host, d.KeyFile(), d.KeyDir())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create router host")
	}

	r := &Router{host: d}

	if err := r.host.Connect(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to connect to router")
	}

	return r, nil
}

// Prepare initial test AP state (e.g., initializing wiphy/wdev).
func (r *Router) Initialize(ctx context.Context) error {
	wiphys, err := r.getWiphys(ctx)
	if err != nil {
		return err
	}
	wdevs, err := r.getWdevs(ctx)
	if err != nil {
		return err
	}

	for _, w := range wdevs {
		testing.ContextLogf(ctx, "Deleting wdev %s on router", w)
		if out, err := r.host.Run(ctx, fmt.Sprintf("iw dev %s del", w)); err != nil {
			return errors.Wrapf(err, "failed to delete wdev %s: %s", w, string(out))
		}
	}

	for i, p := range wiphys {
		w := fmt.Sprintf("managed%d", i)
		testing.ContextLogf(ctx, "Creating wdev %s on wiphy %s", w, p)
		if out, err := r.host.Run(ctx, fmt.Sprintf("iw phy %s interface add %s type managed", p, w)); err != nil {
			return errors.Wrapf(err, "failed to create wdev %s on wiphy %s: %s", w, p, string(out))
		}

		r.ifaces = append(r.ifaces, wDev{name: w, wiphy: p})
	}

	return nil
}

// Return the name of the Nth wdev configured on this AP.
func (r *Router) GetAPWdev(idx int) (string, error) {
	if idx >= len(r.ifaces) {
		return "", errors.New(fmt.Sprintf("not enough wdevs available (%d >= %d)", idx, len(r.ifaces)))
	}
	return r.ifaces[idx].name, nil
}

func (r *Router) getWiphys(ctx context.Context) ([]string, error) {
	out, err := r.host.Run(ctx, "ls -1 /sys/class/ieee80211/")
	if err != nil {
		return nil, errors.Wrap(err, "failed to list wiphys")
	}

	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}

func (r *Router) getWdevs(ctx context.Context) ([]string, error) {
	out, err := r.host.Run(ctx, "iw dev")
	if err != nil {
		return nil, errors.Wrap(err, "failed to list wdevs")
	}

	wdevs := make([]string, 0)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && fields[0] == "Interface" {
			wdevs = append(wdevs, fields[1])
		}
	}

	return wdevs, nil
}

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
