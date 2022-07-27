// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package toolkit

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	factoryservice "chromiumos/tast/services/cros/factory"
	"chromiumos/tast/testing"
)

const (
	// ToolkitServiceDep is the string value that requires Tests/Fixtures
	// that uses the method in this package to add in their `ServiceDeps`.
	ToolkitServiceDep = "tast.cros.factory.Toolkit"
)

// InstallFactoryToolKit installs the factory toolkit via RCP call. The
// installation does not start factory processes automatically, thus goofy
// process and other factory service are not running after this call. Tests or
// Fixtures using this method should declare the `ToolkitServiceDep` in the
// `ServiceDeps` field.
func InstallFactoryToolKit(ctx context.Context, dut *dut.DUT, RPCHint *testing.RPCHint, noEnable bool) (string, error) {
	client, conn, err := createFactoryServiceClient(ctx, dut, RPCHint)
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)

	req := factoryservice.InstallRequest{NoEnable: noEnable}
	res, err := client.Install(ctx, &req)
	if err != nil {
		return "", err
	}
	return res.Version, nil
}

// UninstallFactoryToolKit uninstalls the factory toolkit via RCP call. The
// uninstallation does not stop factory processes, and the recommended way of
// uninstalling the factory toolkit is to first terminate these processes, then
// call this method. Tests or Fixtures using this method should declare the
// `ToolkitServiceDep` in the `ServiceDeps` field.
func UninstallFactoryToolKit(ctx context.Context, dut *dut.DUT, RPCHint *testing.RPCHint) error {
	client, conn, err := createFactoryServiceClient(ctx, dut, RPCHint)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = client.Uninstall(ctx, &factoryservice.UninstallRequest{})
	return err
}

func createFactoryServiceClient(ctx context.Context, dut *dut.DUT, RPCHint *testing.RPCHint) (factoryservice.ToolkitClient, *rpc.Client, error) {
	conn, err := rpc.Dial(ctx, dut, RPCHint)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	client := factoryservice.NewToolkitClient(conn.Conn)
	return client, conn, nil
}
