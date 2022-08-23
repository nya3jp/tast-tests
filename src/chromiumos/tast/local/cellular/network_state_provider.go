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

// NetworkStateProvider provides methods to query and manipulate collections of cellular networks.
type NetworkStateProvider struct {
	euicc   *hermes.EUICC
	manager *shill.Manager
}

// NewNetworkStateProvider sets up the manager items required to fetch cellular network information.
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

type pSimNetwork struct {
	iccid   string
	name    string
	service *shill.Service
}

type eSimNetwork struct {
	iccid   string
	eid     string
	profile *hermes.Profile
}

// Properties implements NetworkState.Properties.
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

// Properties implements NetworkState.Properties.
func (p *pSimNetwork) Properties(ctx context.Context) (*NetworkStateProperties, error) {
	return &NetworkStateProperties{
		Eid:   "",
		Iccid: p.iccid,
		Name:  p.name,
	}, nil
}

// ESimNetworkStates returns the available eSIM networks.
func (n *NetworkStateProvider) ESimNetworkStates(ctx context.Context) ([]NetworkState, error) {
	profiles, err := n.euicc.InstalledProfiles(ctx, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get installed profiles from hermes")
	}

	eid, err := n.euicc.Eid(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get eUICC EID")
	}

	networks := make([]NetworkState, 0, len(profiles))
	for idx := range profiles {
		profile := &profiles[idx]
		iccid, err := profile.Iccid(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get profile ICCID")
		}

		networks = append(networks, &eSimNetwork{
			iccid:   iccid,
			eid:     eid,
			profile: profile,
		})
	}

	return networks, nil
}

// PSimNetworkStates returns the available pSIM networks.
func (n *NetworkStateProvider) PSimNetworkStates(ctx context.Context) ([]NetworkState, error) {
	services, _, err := n.manager.ServicesByTechnology(ctx, shill.TechnologyCellular)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get available services from Manager")
	}

	networks := make([]NetworkState, 0, len(services))
	for _, service := range services {
		if visible, err := service.IsVisible(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to check if service is visible")
		} else if !visible {
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
			return nil, errors.Wrap(err, "failed to get network name")
		}

		if eid == "" && iccid != "" {
			networks = append(networks, &pSimNetwork{
				iccid:   iccid,
				name:    name,
				service: service,
			})
		}
	}
	return networks, nil
}

// NetworkStates returns the currently available cellular networks.
func (n *NetworkStateProvider) NetworkStates(ctx context.Context) ([]NetworkState, error) {
	pSims, err := n.PSimNetworkStates(ctx)
	if err != nil {
		return nil, err
	}

	eSims, err := n.ESimNetworkStates(ctx)
	if err != nil {
		return nil, err
	}

	states := make([]NetworkState, 0, len(pSims)+len(eSims))
	states = append(states, eSims...)
	states = append(states, pSims...)
	return states, nil
}

// PSimNetworkNames returns the names of the currently available pSIM networks.
func (n *NetworkStateProvider) PSimNetworkNames(ctx context.Context) ([]string, error) {
	networks, err := n.PSimNetworkStates(ctx)
	if err != nil {
		return nil, err
	}

	return fetchNetworkNames(ctx, networks)
}

// ESimNetworkNames returns the names of the currently available eSIM networks.
func (n *NetworkStateProvider) ESimNetworkNames(ctx context.Context) ([]string, error) {
	networks, err := n.ESimNetworkStates(ctx)
	if err != nil {
		return nil, err
	}

	return fetchNetworkNames(ctx, networks)
}

func fetchNetworkNames(ctx context.Context, networks []NetworkState) ([]string, error) {
	names := make([]string, len(networks))
	for i, network := range networks {
		properties, err := network.Properties(ctx)
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
			return errors.Errorf("failed to make all network names unique, duplicate pSIM networks: %q found", name)
		}
		seen[name] = true
	}

	profiles, err := n.euicc.InstalledProfiles(ctx, true)
	if err != nil {
		return errors.Wrap(err, "failed to fetch installed profiles from hermes")
	}

	c := 1
	for _, profile := range profiles {
		for seen[fmt.Sprintf("%s%d", prefix, c)] {
			c++
		}

		name := fmt.Sprintf("%s%d", prefix, c)
		if err := profile.Rename(ctx, name); err != nil {
			return errors.Wrap(err, "failed to rename eSIM profile")
		}
		c++
	}

	return nil
}
