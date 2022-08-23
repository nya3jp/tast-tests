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

// NetworkProvider provides methods to query and manipulate collections of cellular networks.
type NetworkProvider interface {
	// ESimNetworks returns the available eSIM networks.
	ESimNetworks(ctx context.Context) ([]ESimNetwork, error)
	// PSimNetworks returns the available pSIM networks.
	PSimNetworks(ctx context.Context) ([]Network, error)
	// Networks returns the currently available cellular networks.
	Networks(ctx context.Context) ([]Network, error)
	// PSimNetworkNames returns the names of the currently available eSIM networks.
	ESimNetworkNames(ctx context.Context) ([]string, error)
	// PSimNetworkNames returns the names of the currently available pSIM networks.
	PSimNetworkNames(ctx context.Context) ([]string, error)
	// NetworkNames returns the currently available cellular network names.
	NetworkNames(ctx context.Context) ([]string, error)
	// RenameESimProfiles renames all eSIM profiles to be unique and follow the pattern: prefix1, prefix2 .... prefixN.
	RenameESimProfiles(ctx context.Context, prefix string) error
}

// networkProvider is a type to which a pointer implements NetworkProvider.
type networkProvider struct {
	euicc   *hermes.EUICC
	manager *shill.Manager
}

// NewNetworkProvider sets up the manager items required to fetch cellular network information.
func NewNetworkProvider(ctx context.Context, useTestEuicc bool) (NetworkProvider, error) {
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

	return &networkProvider{
		euicc:   euicc,
		manager: manager,
	}, nil
}

// NetworkProperties represents the current properties of a cellular network.
type NetworkProperties struct {
	Eid   string
	Iccid string
	Name  string
}

// Network represents a cellular network whose properties can be queried.
type Network interface {
	// Properties fetches the current cellular network properties.
	Properties(ctx context.Context) (*NetworkProperties, error)
}

// ESimNetwork represents an eSIM cellular network.
type ESimNetwork interface {
	Network
	// Profile returns the profile D-BUS object representing the underlying eSIM profile.
	Profile() *hermes.Profile
}

// eSimNetwork is a type to which a pointer implements ESimNetwork.
type eSimNetwork struct {
	iccid   string
	eid     string
	profile *hermes.Profile
}

// pSimNetwork is a type to which a pointer implements Network.
type pSimNetwork struct {
	iccid   string
	name    string
	service *shill.Service
}

// Properties implements Network.Properties.
func (e *eSimNetwork) Properties(ctx context.Context) (*NetworkProperties, error) {
	name, err := e.profile.Nickname(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get profile name")
	}

	if name == "" {
		name, err = e.profile.ServiceProvider(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get profile service provider")
		}
	}

	return &NetworkProperties{
		Eid:   e.eid,
		Iccid: e.iccid,
		Name:  name,
	}, nil
}

// Profile implements ESimNetwork.Profile.
func (e *eSimNetwork) Profile() *hermes.Profile {
	return e.profile
}

// Properties implements Network.Properties.
func (p *pSimNetwork) Properties(ctx context.Context) (*NetworkProperties, error) {
	return &NetworkProperties{
		Eid:   "",
		Iccid: p.iccid,
		Name:  p.name,
	}, nil
}

// ESimNetworks implements NetworkProvider.ESimNetworks.
func (n *networkProvider) ESimNetworks(ctx context.Context) ([]ESimNetwork, error) {
	profiles, err := n.euicc.InstalledProfiles(ctx, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get installed profiles from hermes")
	}

	eid, err := n.euicc.Eid(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get eUICC EID")
	}

	networks := make([]ESimNetwork, 0, len(profiles))
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

// PSimNetworks implements NetworkProvider.PSimNetworks.
func (n *networkProvider) PSimNetworks(ctx context.Context) ([]Network, error) {
	services, _, err := n.manager.ServicesByTechnology(ctx, shill.TechnologyCellular)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get available services from Manager")
	}

	networks := make([]Network, 0, len(services))
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
			return nil, errors.Wrap(err, "failed to get service name")
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

// Networks implements NetworkProvider.Networks.
func (n *networkProvider) Networks(ctx context.Context) ([]Network, error) {
	pSims, err := n.PSimNetworks(ctx)
	if err != nil {
		return nil, err
	}

	eSims, err := n.ESimNetworks(ctx)
	if err != nil {
		return nil, err
	}

	networks := make([]Network, 0, len(pSims)+len(eSims))
	for _, sim := range eSims {
		networks = append(networks, sim)
	}

	networks = append(networks, pSims...)
	return networks, nil
}

// ESimNetworkNames implements NetworkProvider.ESimNetworkNames.
func (n *networkProvider) ESimNetworkNames(ctx context.Context) ([]string, error) {
	networks, err := n.ESimNetworks(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(networks))
	for i, network := range networks {
		properties, err := network.Properties(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get network properties")
		}
		names[i] = properties.Name
	}

	return names, nil
}

// PSimNetworkNames implements NetworkProvider.PSimNetworkNames.
func (n *networkProvider) PSimNetworkNames(ctx context.Context) ([]string, error) {
	networks, err := n.PSimNetworks(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(networks))
	for i, network := range networks {
		properties, err := network.Properties(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get network properties")
		}
		names[i] = properties.Name
	}

	return names, nil
}

// NetworkNames implements NetworkProvider.NetworkNames.
func (n *networkProvider) NetworkNames(ctx context.Context) ([]string, error) {
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

// RenameESimProfiles implements NetworkNames.RenameESimProfiles.
//
// Note: pSIM network names cannot be changed but are still checked when changing the network names.
func (n *networkProvider) RenameESimProfiles(ctx context.Context, prefix string) error {
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

	eSimNetworks, err := n.ESimNetworks(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get eSIM networks")
	}

	c := 1
	for _, network := range eSimNetworks {
		for seen[fmt.Sprintf("%s%d", prefix, c)] {
			c++
		}

		name := fmt.Sprintf("%s%d", prefix, c)
		if err := network.Profile().Rename(ctx, name); err != nil {
			return errors.Wrap(err, "failed to rename eSIM profile")
		}
		c++
	}

	return nil
}
