// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servers defines utilitizes that allow tests to access various
// servers, such as cache and provisions, provided in the lab or
// development environment.
package servers

import (
	"sync"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ServerType defines the supported server types of this utility.
type ServerType int

// The specific server type for each server.
const (
	Servo = iota
	Provision
	DUT
	Cache
)

// Provide constant for primary and default companion DUTs roles for tests to use.
const (
	Primary                  = "primary" // Name for primary role.
	DefaultCompanionDUT1Role = "cd1"     // Default role name of first companion DUT.
	DefaultCompanionDUT2Role = "cd2"     // Default role name of second companion DUT.
	DefaultCompanionDUT3Role = "cd3"     // Default role name of third companion DUT.
)

var (
	// servoServers is a runtime variable to store host information
	// of servo servers.
	servoServers = testing.RegisterVarString(
		"servers.servo",
		"",
		"A variable to store host information of servo servers")

	// provisionServers is a runtime variable to store host information
	// of provision servers.
	provisionServers = testing.RegisterVarString(
		"servers.provision",
		"",
		"A variable to store host information of provision servers")

	// dutServers is a runtime variable to store host information
	// of DUT servers.
	dutServers = testing.RegisterVarString(
		"servers.dut",
		"",
		"A variable to store host information of DUT servers")

	// cacheServers is a runtime variable to store host information
	// of cache servers.
	cacheServers = testing.RegisterVarString(
		"servers.cache",
		"",
		"A variable to store host information of cache servers")
)

// Server returns the host target information of a server.
// If a server is not available for a particular role, the boolean parameter will
// be returned as false. Otherwise, it will be true.
// This function should not be called from the init functions of tests.
func Server(serverType ServerType, role string) (string, bool, error) {
	allServer, err := allServerHosts()
	if err != nil {
		return "", false, err
	}
	hosts := allServer[serverType]
	if hosts == nil {
		return "", false, nil
	}
	h, ok := hosts[role]
	return h, ok, nil
}

// Servers returns a map of role to host information of a particular server type.
// This function should not be called from the init functions of tests.
func Servers(serverType ServerType) (map[string]string, error) {
	allServer, err := allServerHosts()
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	src, _ := allServer[serverType]
	for role, host := range src {
		result[role] = host
	}
	return result, nil
}

var allServers map[ServerType](map[string]string)
var mtx sync.Mutex

func allServerHosts() (map[ServerType](map[string]string), error) {
	mtx.Lock()
	defer mtx.Unlock()
	if allServers != nil {
		return allServers, nil
	}
	allServers = make(map[ServerType](map[string]string))
	hosts, err := testing.ParseServerVarValues(servoServers.Value())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse servo information")
	}
	allServers[Servo] = hosts

	hosts, err = testing.ParseServerVarValues(provisionServers.Value())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse provision server information")
	}
	allServers[Provision] = hosts

	hosts, err = testing.ParseServerVarValues(dutServers.Value())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse DUT server information")
	}
	allServers[DUT] = hosts

	hosts, err = testing.ParseServerVarValues(cacheServers.Value())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse cache server information")
	}
	allServers[Cache] = hosts
	return allServers, nil
}
