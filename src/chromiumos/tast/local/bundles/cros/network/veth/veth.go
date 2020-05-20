// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package veth contains utility functions for establishing virtual Ethernet pairs.
package veth

import (
	"context"
	"net"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/ip"
	"chromiumos/tast/testing"
)

// Pair represents a Linux pair of virtual Ethernet (veth) devices. Veth devices come in pairs,
// a primary and a peer, representing two sides of a virtual link.
type Pair struct {
	Iface     *net.Interface
	PeerIface *net.Interface
}

// NewPair sets up a new Pair object, with interface iface and peer interface peerIface.
// It removes any existing links of the same name.
func NewPair(ctx context.Context, iface, peerIface string) (*Pair, error) {
	ipr := ip.NewRunner()

	// Delete any existing links.
	for _, name := range []string{iface, peerIface} {
		// Check if interface 'name' exists.
		if _, err := net.InterfaceByName(name); err == nil {
			testing.ContextLogf(ctx, "Deleting existing interface %s", name)
			if err := ipr.DeleteLink(ctx, name); err != nil {
				return nil, errors.Errorf("failed to delete existing link %q", name)
			}
		}
	}

	// Create veth pair.
	if err := ipr.AddLink(ctx, iface, "veth", "peer", "name", peerIface); err != nil {
		return nil, errors.Wrapf(err, "failed to add veth interfaces %q/%q", iface, peerIface)
	}

	i, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get interface %q", iface)
	}

	p, err := net.InterfaceByName(peerIface)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get peer interface %q", peerIface)
	}

	return &Pair{
		Iface:     i,
		PeerIface: p,
	}, nil
}

// Delete deletes the virtual link.
func (v *Pair) Delete(ctx context.Context) error {
	// Only need to delete one end of the pair.
	ipr := ip.NewRunner()
	if err := ipr.DeleteLink(ctx, v.Iface.Name); err != nil {
		return errors.Wrapf(err, "failed to delete veth iface %q", v.Iface.Name)
	}
	return nil
}
