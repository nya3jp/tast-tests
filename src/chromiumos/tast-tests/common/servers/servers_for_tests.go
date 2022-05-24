// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servers defines utilitizes that allow tests to access various
// servers, such as servo and provisions, provided in the lab or
// development environment.
package servers

import (
	"strings"
	"sync"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ServerType defines the supported server types of this utility.
type ServerType int

// The specific server type for each server.
const (
	Servo ServerType = iota
	Provision
	DUT
)

// Provide constant for primary DUTs roles for tests to use.
// Note: currently, we use "cd1", "cd2", ... for companinon DUT roles.
//       There is no process to allow customization it yet.
const (
	Primary = "" // Name for primary role.
)

// ErrNotFound is used the requested server information was not specified on command line.
var ErrNotFound = errors.New("cannot find server information")

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
)

// Server returns the host target information of a server.
// This function should not be called from the init functions of tests.
func Server(serverType ServerType, role string) (string, error) {
	var serverTypeName string
	switch serverType {
	case Servo:
		serverTypeName = "servo"
	case Provision:
		serverTypeName = "provision server"
	case DUT:
		serverTypeName = "DUT server"
	}
	allServer, err := allServerHosts()
	if err != nil {
		return "", errors.Wrapf(
			err, "failed to find %s for role %s", serverTypeName, role)
	}
	hosts := allServer[serverType]
	if hosts == nil {
		return "", errors.Wrapf(
			ErrNotFound, "could not find %s for role %s", serverTypeName, role)
	}
	return hosts[role], nil
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

var (
	allServers map[ServerType](map[string]string)
	once       sync.Once
	parseErr   error
)

func allServerHosts() (map[ServerType](map[string]string), error) {
	once.Do(func() {
		servoHosts, err := parseServerVarValues(servoServers.Value())
		if err != nil {
			parseErr = errors.Wrap(err, "failed to parse servo information")
			return
		}
		provisionServerHosts, err := parseServerVarValues(provisionServers.Value())
		if err != nil {
			parseErr = errors.Wrap(err, "failed to parse provision server information")
			return
		}
		dutServerHosts, err := parseServerVarValues(dutServers.Value())
		if err != nil {
			parseErr = errors.Wrap(err, "failed to parse DUT server information")
			return
		}
		allServers = map[ServerType](map[string]string){
			Servo:     servoHosts,
			Provision: provisionServerHosts,
			DUT:       dutServerHosts,
		}
	})
	return allServers, parseErr
}

// parseServerVarValues parse the values of server related run time variables
// and return a role to server host map.
// Example input: ":addr1:22,cd1:addr2:2222"
// Example output: { "": "addr1:22", "cd1": "addr2:2222" }
func parseServerVarValues(inputValue string) (result map[string]string, err error) {
	result = make(map[string]string)
	if len(inputValue) == 0 {
		return result, nil
	}
	serverInfos := strings.Split(inputValue, ",")
	for _, serverInfo := range serverInfos {
		roleHost := strings.SplitN(serverInfo, ":", 2)
		if len(roleHost) != 2 {
			return nil, errors.Errorf("invalid role/server value %s", serverInfo)
		}
		result[roleHost[0]] = roleHost[1]
	}
	return result, nil
}
