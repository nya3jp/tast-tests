// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"fmt"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/shill"
)

// NetworkStateProvider provides methods to bulk query and modify information about currently available pSIM and eSIM networks.
type NetworkStateProvider struct {
	euicc   *hermes.EUICC
	manager *shill.Manager
}

// NewNetworkStateProvider sets up the shill and hermes items required to fetch the pSIM and eSIM network information.
func NewNetworkStateProvider(ctx context.Context, useTestEuicc bool) (*NetworkStateProvider, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager")
	}

	euicc, _, err := hermes.GetEUICC(ctx, useTestEuicc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create eUICC")
	}

	if err := euicc.DBusObject.Call(ctx, hermesconst.EuiccMethodUseTestCerts, useTestEuicc).Err; err != nil {
		return nil, errors.Wrap(err, "failed to set use test cert on eUICC")
	}

	return &NetworkStateProvider{
		euicc:   euicc,
		manager: manager,
	}, nil
}

// NetworkStateProperties represents the current properties of a cellular network.
type NetworkStateProperties struct {
	Eid   string
	Iccid string
	Name  string
}

// NetworkState represents a cellular network whose properties can be queried.
type NetworkState interface {
	// Properties fetches the current cellular network properties.
	Properties(ctx context.Context) (*NetworkStateProperties, error)
}

// ESimNetworkState represents an eSIM cellular network.
type ESimNetworkState interface {
	NetworkState
	// Profile returns the profile D-BUS object representing the underlying eSIM profile.
	Profile() *hermes.Profile
}

type eSimNetwork struct {
	iccid   string
	eid     string
	profile *hermes.Profile
}

// Properties implements Network.Properties.
func (e *eSimNetwork) Properties(ctx context.Context) (*NetworkStateProperties, error) {
	name, err := e.profile.Nickname(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get SIM name")
	}

	if name == "" {
		name, err = e.profile.ServiceProvider(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get profile service provider")
		}
	}

	return &NetworkStateProperties{
		Eid:   e.eid,
		Iccid: e.iccid,
		Name:  name,
	}, nil
}

// Properties implements ESimNetwork.Profile.
func (e *eSimNetwork) Profile() *hermes.Profile {
	return e.profile
}

type pSimNetwork struct {
	eid     string
	iccid   string
	name    string
	service *shill.Service
}

// Properties implements Network.Properties.
func (p *pSimNetwork) Properties(ctx context.Context) (*NetworkStateProperties, error) {
	return &NetworkStateProperties{
		Eid:   p.eid,
		Iccid: p.iccid,
		Name:  p.name,
	}, nil
}

// ESimNetworks returns the available eSIM networks.
func (n *NetworkStateProvider) ESimNetworks(ctx context.Context) ([]ESimNetworkState, error) {
	profiles, err := n.euicc.InstalledProfiles(ctx, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch installed profiles from hermes")
	}

	eid, err := n.euicc.Eid(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get EID from eUICC")
	}

	res := make([]ESimNetworkState, 0, len(profiles))
	for idx := range profiles {
		profile := &profiles[idx]
		iccid, err := profile.Iccid(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get profile iccid")
		}

		res = append(res, &eSimNetwork{
			iccid:   iccid,
			eid:     eid,
			profile: profile,
		})
	}

	return res, nil
}

// PSimNetworks returns the available pSIM networks.
func (n *NetworkStateProvider) PSimNetworks(ctx context.Context) ([]NetworkState, error) {
	services, _, err := n.manager.ServicesByTechnology(ctx, shill.TechnologyCellular)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get services")
	}

	res := make([]NetworkState, 0, len(services))
	for _, service := range services {
		visible, err := service.IsVisible(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if service is visible")
		}
		if !visible {
			continue
		}

		eid, err := service.GetEid(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get service eid")
		}
		iccid, err := service.GetIccid(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get service iccid")
		}

		name, err := service.GetName(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get SIM name")
		}

		if eid == "" && iccid != "" {
			res = append(res, &pSimNetwork{
				iccid:   iccid,
				eid:     eid,
				name:    name,
				service: service,
			})
		}
	}
	return res, nil
}

// Networks returns the currently available cellular networks.
func (n *NetworkStateProvider) Networks(ctx context.Context) ([]NetworkState, error) {
	pSims, err := n.PSimNetworks(ctx)
	if err != nil {
		return nil, err
	}

	eSims, err := n.ESimNetworks(ctx)
	if err != nil {
		return nil, err
	}

	infos := make([]NetworkState, 0, len(pSims)+len(eSims))
	for _, sim := range eSims {
		infos = append(infos, sim)
	}

	infos = append(infos, pSims...)
	return infos, nil
}

// PSimNetworkNames returns the names of the currently available pSIM networks.
func (n *NetworkStateProvider) PSimNetworkNames(ctx context.Context) ([]string, error) {
	netInfo, err := n.PSimNetworks(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(netInfo))
	for i, sim := range netInfo {
		properties, err := sim.Properties(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get network name")
		}
		names[i] = properties.Name
	}

	return names, nil
}

// ESimNetworkNames returns the names of the currently available eSIM networks.
func (n *NetworkStateProvider) ESimNetworkNames(ctx context.Context) ([]string, error) {
	netInfo, err := n.ESimNetworks(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(netInfo))
	for i, sim := range netInfo {
		properties, err := sim.Properties(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get network name")
		}
		names[i] = properties.Name
	}

	return names, nil
}

// NetworkNames returns the currently available cellular network names.
func (n *NetworkStateProvider) NetworkNames(ctx context.Context) ([]string, error) {
	pSims, err := n.PSimNetworkNames(ctx)
	if err != nil {
		return nil, err
	}

	eSims, err := n.ESimNetworkNames(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(pSims)+len(eSims))
	names = append(names, eSims...)
	names = append(names, pSims...)
	return names, nil
}

// RenameESimProfiles renames all eSIM profiles to be unique and follow the pattern: prefix1, prefix2 .... prefixN.
//
// Note: pSIM network names cannot be changed but are still be checked when changing the network names.
func (n *NetworkStateProvider) RenameESimProfiles(ctx context.Context, prefix string) error {
	pSimNames, err := n.PSimNetworkNames(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get pSIM network names")
	}

	seen := make(map[string]bool)
	for _, name := range pSimNames {
		if seen[name] {
			return errors.Errorf("failed to make all network names unique, duplicate pSIM networks: %s found", name)
		}
		seen[name] = true
	}

	simInfos, err := n.ESimNetworks(ctx)
	if err != nil {
		return err
	}

	c := 1
	for _, sim := range simInfos {
		for seen[fmt.Sprintf("%s%d", prefix, c)] {
			c++
		}

		name := fmt.Sprintf("%s%d", prefix, c)
		if err := sim.Profile().Rename(ctx, name); err != nil {
			return errors.Wrap(err, "failed to rename eSIM profile")
		}
		c++
	}

	return nil
}
