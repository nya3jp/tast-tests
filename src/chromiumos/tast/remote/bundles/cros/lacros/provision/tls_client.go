// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package provision

import (
	"context"

	"go.chromium.org/chromiumos/config/go/api/test/tls"
	"go.chromium.org/chromiumos/config/go/api/test/tls/dependencies/longrunning"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TLSAddrVar is a variable to set the address of TLS for provisioning.
var TLSAddrVar = testing.RegisterVarString(
	"provision.TLSAddr",
	"10.254.254.254:7152", // The default address
	"The address {host:port} of TLS for provisioning",
)

// TLSClient holds a gRPC connection and address of TLS.
type TLSClient struct {
	conn *grpc.ClientConn
	addr string
}

// Dial connects to a given gRPC service. Close should be called to release resources.
func Dial(ctx context.Context, addr string) (*TLSClient, error) {
	testing.ContextLog(ctx, "Connecting to TLS at ", addr)
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to TLS at %v,", addr)
	}
	return &TLSClient{
		conn: conn,
		addr: addr,
	}, nil
}

// Close releases resources. This should be called after using TLSClient.
func (c *TLSClient) Close() error {
	return c.conn.Close()
}

// ProvisionLacrosFromDeviceFile calls a TLS ProvisionLacros API to provision from the device image path.
func (c *TLSClient) ProvisionLacrosFromDeviceFile(ctx context.Context, dutName, deviceFilePath, overrideVersion, overrideInstallPath string) error {
	req := tls.ProvisionLacrosRequest{
		Name: dutName,
		Image: &tls.ProvisionLacrosRequest_LacrosImage{
			PathOneof: &tls.ProvisionLacrosRequest_LacrosImage_DeviceFilePrefix{
				DeviceFilePrefix: deviceFilePath, // "file:///opt/google/lacros"
			},
		},
		OverrideVersion:     overrideVersion,
		OverrideInstallPath: overrideInstallPath, // "/home/chronos/cros-components/lacros-dogfood-dev"
	}

	t := tls.NewCommonClient(c.conn)
	op, err := t.ProvisionLacros(ctx, &req)
	if err != nil {
		return errors.Wrapf(err, "ProvisionLacros: failed to call a RPC with %v", req)
	}
	opName := op.Name
	lro := longrunning.NewOperationsClient(c.conn)
	op, err = lro.WaitOperation(ctx, &longrunning.WaitOperationRequest{
		Name: opName,
	})
	if err != nil {
		return errors.Wrapf(err, "ProvisionLacros: failed to wait an operation: %v", op)
	}
	return nil
}
