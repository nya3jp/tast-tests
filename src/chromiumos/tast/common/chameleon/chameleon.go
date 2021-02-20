// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chameleon is used to communicate with chameleon devices connected to DUTs.
// It communicates with chameleon over XML-RPC.
package chameleon

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

// Chameleon holds the chameleond connection information.
type Chameleon struct {
	host   string
	port   int
	xmlrpc *servo.XMLRpc
}

const (
	// chameleondDefaultHost is the default host for chameleond.
	chameleondDefaultHost = "localhost"
	// chameleondDefaultPort is the default port for chameleond.
	chameleondDefaultPort = 9992
)

// New creates a new Servo object for communicating with a chameleond instance.
// connSpec holds chameleond's location, either as "host:port" or just "host"
// (to use the default port).
func New(ctx context.Context, connSpec string) (*Chameleon, error) {
	testing.ContextLogf(ctx, "New chameloen - conSpec: %s", connSpec)
	host, port, err := parseConnSpec(connSpec)
	if err != nil {
		return nil, err
	}
	r := &servo.XMLRpc{Ctx: ctx, Host: host, Port: port}
	s := &Chameleon{host: host, port: port, xmlrpc: r}

	return s, nil
}

// Default creates a Chameleon object for communicating with a local chameleond
// instance using the default port.
func Default(ctx context.Context) (*Chameleon, error) {
	connSpec := fmt.Sprintf("%s:%d", chameleondDefaultHost, chameleondDefaultPort)
	return New(ctx, connSpec)
}

// parseConnSpec parses a connection host:port string and returns the
// components.
func parseConnSpec(c string) (host string, port int, err error) {
	if len(c) == 0 {
		return "", 0, errors.New("got empty string")
	}

	parts := strings.Split(c, ":")
	if len(parts) == 1 {
		// If no port, return default port.
		return parts[0], chameleondDefaultPort, nil
	}
	if len(parts) == 2 {
		port, err = strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, errors.Errorf("got invalid port int in spec %q", c)
		}
		return parts[0], port, nil
	}

	return "", 0, errors.Errorf("got invalid connection spec %q", c)
}
