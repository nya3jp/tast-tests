// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/testing"
)

// IfaceManager manages WiFi ifaces on a controller router and tracks their availability.
type IfaceManager struct {
	r           support.Router
	iwr         *iw.Runner
	nextIfaceID int

	// Available is a map from the interface name to iw.NetDev for available interfaces.
	Available map[string]*iw.NetDev

	// Busy is a map from the interface name to iw.NetDev for busy interfaces.
	Busy map[string]*iw.NetDev
}

// NewRouterIfaceManager creates a new IfaceManager
func NewRouterIfaceManager(r support.Router, iwr *iw.Runner) *IfaceManager {
	return &IfaceManager{
		r:         r,
		iwr:       iwr,
		Available: make(map[string]*iw.NetDev),
		Busy:      make(map[string]*iw.NetDev),
	}
}

// RemoveAll removes all WiFi interfaces.
func (im *IfaceManager) RemoveAll(ctx context.Context) error {
	netDevs, err := im.iwr.ListInterfaces(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list interfaces")
	}
	for _, w := range netDevs {
		if err := im.Remove(ctx, w.IfName); err != nil {
			return err
		}
	}
	return nil
}

// Remove removes the given iface with the iw command.
func (im *IfaceManager) Remove(ctx context.Context, iface string) error {
	testing.ContextLogf(ctx, "Deleting wdev %s on %s", iface, im.r.RouterName())
	if err := im.iwr.RemoveInterface(ctx, iface); err != nil {
		return err
	}
	if _, ok := im.Available[iface]; ok {
		delete(im.Available, iface)
	}
	if _, ok := im.Busy[iface]; ok {
		delete(im.Busy, iface)
	}
	return nil
}

// uniqueIfaceName returns an unique name for interface with type t.
func (im *IfaceManager) uniqueIfaceName(t iw.IfType) string {
	name := fmt.Sprintf("%s%d", string(t), im.nextIfaceID)
	im.nextIfaceID++
	return name
}

// Create creates an interface on phy of type t and returns the name of created interface.
func (im *IfaceManager) Create(ctx context.Context, phyName string, phyID int, t iw.IfType) (*iw.NetDev, error) {
	ifaceName := im.uniqueIfaceName(t)
	testing.ContextLogf(ctx, "Creating wdev %s on wiphy %s", ifaceName, phyName)
	if err := im.iwr.AddInterface(ctx, phyName, ifaceName, t); err != nil {
		return nil, err
	}
	nd := &iw.NetDev{
		PhyNum: phyID,
		IfName: ifaceName,
		IfType: t,
	}
	im.Available[ifaceName] = nd
	return nd, nil
}

// SetBusy marks iface as busy.
func (im *IfaceManager) SetBusy(iface string) {
	nd, ok := im.Available[iface]
	if !ok {
		return
	}
	im.Busy[iface] = nd
	delete(im.Available, iface)
}

// SetAvailable marks iface as available.
func (im *IfaceManager) SetAvailable(iface string) {
	nd, ok := im.Busy[iface]
	if !ok {
		return
	}
	im.Available[iface] = nd
	delete(im.Busy, iface)
}

// IsPhyBusyAny returns true if the phyID is occupied by a busy interface of any type.
func (im *IfaceManager) IsPhyBusyAny(phyID int) bool {
	for _, nd := range im.Busy {
		if nd.PhyNum == phyID {
			return true
		}
	}
	return false
}

// IsPhyBusy returns true if the phyID is occupied by a busy interface of type t.
func (im *IfaceManager) IsPhyBusy(phyID int, t iw.IfType) bool {
	for _, nd := range im.Busy {
		if nd.PhyNum == phyID && nd.IfType == t {
			return true
		}
	}
	return false
}
