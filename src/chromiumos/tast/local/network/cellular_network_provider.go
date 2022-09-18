// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/shill"
)

// CellularNetworkProvider provides methods to query and manipulate collections of cellular networks.
type CellularNetworkProvider interface {
	// ESimNetworks returns the available eSIM networks.
	ESimNetworks(ctx context.Context) ([]ESimCellularNetwork, error)
	// PSimNetworks returns the available pSIM networks.
	PSimNetworks(ctx context.Context) ([]CellularNetwork, error)
	// Networks returns the currently available cellular networks.
	Networks(ctx context.Context) ([]CellularNetwork, error)
	// PSimNetworkNames returns the names of the currently available eSIM networks.
	ESimNetworkNames(ctx context.Context) ([]string, error)
	// PSimNetworkNames returns the names of the currently available pSIM networks.
	PSimNetworkNames(ctx context.Context) ([]string, error)
	// NetworkNames returns the currently available cellular network names.
	NetworkNames(ctx context.Context) ([]string, error)
	// RenameESimProfiles renames all eSIM profiles to be unique and follow the pattern: prefix1, prefix2 .... prefixN.
	RenameESimProfiles(ctx context.Context, prefix string) error
	// Fetches network name by iccid
	GetNetworkNameByIccid(ctx context.Context, iccid string) (string, error)
}

// CellularNetworkProperties represents the current properties of a cellular network.
type CellularNetworkProperties struct {
	Eid   string
	Iccid string
	Name  string
}

// CellularNetwork represents a cellular network whose properties can be queried.
type CellularNetwork interface {
	// Properties fetches the current cellular network properties.
	Properties(ctx context.Context) (*CellularNetworkProperties, error)
}

// ESimCellularNetwork represents an eSIM cellular network.
type ESimCellularNetwork interface {
	CellularNetwork
	// Profile returns the profile D-BUS object representing the underlying eSIM profile.
	Profile() *hermes.Profile
}

// eSimCellularNetwork is a type to which a pointer implements ESimCellularNetwork.
type eSimCellularNetwork struct {
	iccid   string
	eid     string
	profile *hermes.Profile
}

// pSimCellularNetwork is a type to which a pointer implements CellularNetwork.
type pSimCellularNetwork struct {
	iccid   string
	name    string
	service *shill.Service
}

// cellularNetworkProvider is a type to which a pointer implements CellularNetworkProvider.
type cellularNetworkProvider struct {
	euicc   *hermes.EUICC
	manager *shill.Manager
}

// Properties implements CellularNetwork.Properties.
func (e *eSimCellularNetwork) Properties(ctx context.Context) (*CellularNetworkProperties, error) {
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

	return &CellularNetworkProperties{
		Eid:   e.eid,
		Iccid: e.iccid,
		Name:  name,
	}, nil
}

// Profile implements ESimCellularNetwork.Profile.
func (e *eSimCellularNetwork) Profile() *hermes.Profile {
	return e.profile
}

// Properties implements CellularNetwork.Properties.
func (p *pSimCellularNetwork) Properties(ctx context.Context) (*CellularNetworkProperties, error) {
	return &CellularNetworkProperties{
		Eid:   "",
		Iccid: p.iccid,
		Name:  p.name,
	}, nil
}

// NewCellularNetworkProvider sets up the manager items required to fetch cellular network information.
func NewCellularNetworkProvider(ctx context.Context, useTestEuicc bool) (CellularNetworkProvider, error) {
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

	return &cellularNetworkProvider{
		euicc:   euicc,
		manager: manager,
	}, nil
}

// ESimNetworks implements CellularNetworkProvider.ESimNetworks.
func (c *cellularNetworkProvider) ESimNetworks(ctx context.Context) ([]ESimCellularNetwork, error) {
	profiles, err := c.euicc.InstalledProfiles(ctx, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get installed profiles from hermes")
	}

	eid, err := c.euicc.Eid(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get eUICC EID")
	}

	networks := make([]ESimCellularNetwork, 0, len(profiles))
	for idx := range profiles {
		profile := &profiles[idx]
		iccid, err := profile.Iccid(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get profile ICCID")
		}

		networks = append(networks, &eSimCellularNetwork{
			iccid:   iccid,
			eid:     eid,
			profile: profile,
		})
	}

	return networks, nil
}

// PSimNetworks implements CellularNetworkProvider.PSimNetworks.
func (c *cellularNetworkProvider) PSimNetworks(ctx context.Context) ([]CellularNetwork, error) {
	services, _, err := c.manager.ServicesByTechnology(ctx, shill.TechnologyCellular)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get available services from Manager")
	}

	networks := make([]CellularNetwork, 0, len(services))
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
		if eid != "" {
			continue
		}

		iccid, err := service.GetIccid(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get service iccid")
		}
		if iccid == "" {
			continue
		}

		name, err := service.GetName(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get service name")
		}

		networks = append(networks, &pSimCellularNetwork{
			iccid:   iccid,
			name:    name,
			service: service,
		})
	}
	return networks, nil
}

// Networks implements CellularNetworkProvider.Networks.
func (c *cellularNetworkProvider) Networks(ctx context.Context) ([]CellularNetwork, error) {
	networks, err := c.PSimNetworks(ctx)
	if err != nil {
		return nil, err
	}

	eSims, err := c.ESimNetworks(ctx)
	if err != nil {
		return nil, err
	}

	for _, sim := range eSims {
		networks = append(networks, sim)
	}

	return networks, nil
}

// ESimNetworkNames implements CellularNetworkProvider.ESimNetworkNames.
func (c *cellularNetworkProvider) ESimNetworkNames(ctx context.Context) ([]string, error) {
	networks, err := c.ESimNetworks(ctx)
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

// PSimNetworkNames implements CellularNetworkProvider.PSimNetworkNames.
func (c *cellularNetworkProvider) PSimNetworkNames(ctx context.Context) ([]string, error) {
	networks, err := c.PSimNetworks(ctx)
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

// NetworkNames implements CellularNetworkProvider.NetworkNames.
func (c *cellularNetworkProvider) NetworkNames(ctx context.Context) ([]string, error) {
	pSims, err := c.PSimNetworkNames(ctx)
	if err != nil {
		return nil, err
	}

	eSims, err := c.ESimNetworkNames(ctx)
	if err != nil {
		return nil, err
	}

	return append(pSims, eSims...), nil
}

func (c *cellularNetworkProvider) GetNetworkNameByIccid(ctx context.Context, iccid string) (string, error) {
	pSims, err := c.PSimNetworks(ctx)
	if err != nil {
		return "", err
	}

	for _, network := range pSims {
		properties, err := network.Properties(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to get network properties")
		}
		if properties.Iccid == iccid {
			return properties.Name, nil
		}
	}

	eSims, err := c.ESimNetworks(ctx)
	if err != nil {
		return "", err
	}

	for _, network := range eSims {
		properties, err := network.Properties(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to get network properties")
		}
		if properties.Iccid == iccid {
			return properties.Name, nil
		}
	}

	return "", nil
}

// RenameESimProfiles implements CellularNetworkProvider.RenameESimProfiles.
//
// Note: pSIM network names cannot be changed but are still checked when changing the network names.
func (c *cellularNetworkProvider) RenameESimProfiles(ctx context.Context, prefix string) error {
	pSimNames, err := c.PSimNetworkNames(ctx)
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

	eSimNetworks, err := c.ESimNetworks(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get eSIM networks")
	}

	count := 1
	for _, network := range eSimNetworks {
		for seen[fmt.Sprintf("%s%d", prefix, count)] {
			count++
		}

		name := fmt.Sprintf("%s%d", prefix, count)
		if err := network.Profile().Rename(ctx, name); err != nil {
			return errors.Wrap(err, "failed to rename eSIM profile")
		}
		count++
	}

	return nil
}
