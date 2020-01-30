// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package veth contains utility functions for establishing virtual Ethernet pairs.
package veth

import (
	"context"
	"net"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Pair represents a Linux pair of virtual Ethernet (veth) devices. Veth devices come in pairs,
// a primary and a peer, representing two sides of a virtual link.
type Pair struct {
	Iface     string
	PeerIface string
}

// NewPair sets up a new Pair object, with interface |iface| and peer interface |peerIface|.
// It removes any existing links of the same name.
func NewPair(ctx context.Context, iface string, peerIface string) (*Pair, error) {
	v := Pair{
		Iface:     iface,
		PeerIface: peerIface,
	}

	// Delete any existing links.
	for _, name := range []string{v.Iface, v.PeerIface} {
		// Check if interface 'name' exists.
		if _, err := net.InterfaceByName(name); err == nil {
			testing.ContextLogf(ctx, "Deleting existing interface %s", name)
			if err = testexec.CommandContext(ctx, "ip", "link", "del", name).Run(); err != nil {
				return nil, errors.Errorf("failed to delete existing link %q", name)
			}
		}
	}

	// Create veth pair.
	if err := testexec.CommandContext(ctx, "ip", "link", "add", v.Iface, "type", "veth", "peer", "name", v.PeerIface).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrapf(err, "failed to add veth interfaces %q/%q", v.Iface, v.PeerIface)
	}

	return &v, nil
}

// Delete deletes the virtual link.
func (v *Pair) Delete(ctx context.Context) error {
	// Only need to delete one end of the pair.
	if err := testexec.CommandContext(ctx, "ip", "link", "del", v.Iface).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to delete veth iface %q", v.Iface)
	}
	return nil
}
