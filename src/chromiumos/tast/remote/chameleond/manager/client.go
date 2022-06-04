// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manager

import (
	"context"

	pbchameleond "go.chromium.org/chromiumos/config/go/platform/chameleon/chameleond/rpc"
	pbmanager "go.chromium.org/chromiumos/config/go/test/api/test_libs/chameleond_manager"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
)

// ChameleondManagerServiceClient manages a connection to a Chameleond Manager
// gRPC server and provides gRPC clients to its services.
type ChameleondManagerServiceClient struct {
	clientConn               *grpc.ClientConn
	ChameleondService        pbchameleond.ChameleondServiceClient
	ChameleondManagerService pbmanager.ChameleondManagerServiceClient
}

// NewChameleondManagerServiceClient creates a new
// ChameleondManagerServiceClient and establishes a connection to the gRPC
// server at serverAddr.
func NewChameleondManagerServiceClient(ctx context.Context, serverAddr string) (*ChameleondManagerServiceClient, error) {
	c := &ChameleondManagerServiceClient{}
	var err error
	c.clientConn, err = grpc.Dial(serverAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to ChameleondManagerService gRPC server at %q", serverAddr)
	}
	c.ChameleondService = pbchameleond.NewChameleondServiceClient(c.clientConn)
	c.ChameleondManagerService = pbmanager.NewChameleondManagerServiceClient(c.clientConn)
	return c, nil
}

// Close closes the connection to the gRPC server.
func (c *ChameleondManagerServiceClient) Close() error {
	if err := c.clientConn.Close(); err != nil {
		return errors.Wrap(err, "failed to close connection to ChameleondManagerService gRPC server")
	}
	return nil
}
