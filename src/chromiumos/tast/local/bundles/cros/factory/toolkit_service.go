// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"sync"

	"google.golang.org/grpc"

	"chromiumos/tast/local/bundles/cros/factory/toolkit"
	factoryservice "chromiumos/tast/services/cros/factory"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			factoryservice.RegisterToolkitServer(srv, &ToolkitService{
				serviceState: s,
				cmdExecLock:  &sync.Mutex{},
			})
		},
	})
}

// ToolkitService implements tast.cros.factory.Toolkit gRPC service.
type ToolkitService struct {
	serviceState *testing.ServiceState
	cmdExecLock  *sync.Mutex
}

// Install installs the factory toolkit with the arguments specified in the
// request.
func (t *ToolkitService) Install(ctx context.Context, req *factoryservice.InstallRequest) (*factoryservice.InstallResponse, error) {
	t.cmdExecLock.Lock()
	defer t.cmdExecLock.Unlock()

	installer := toolkit.Installer{
		InstallerPath: toolkit.ToolkitInstallerPath,
		NoEnable:      req.NoEnable,
	}
	version, err := installer.InstallFactoryToolKit(ctx)
	if err != nil {
		return nil, err
	}
	return &factoryservice.InstallResponse{
		Version: version,
	}, nil
}

// Uninstall calls the command factory_uninstall.
func (t *ToolkitService) Uninstall(ctx context.Context, _ *factoryservice.UninstallRequest) (*factoryservice.UninstallResponse, error) {
	t.cmdExecLock.Lock()
	defer t.cmdExecLock.Unlock()

	err := toolkit.UninstallFactoryToolKit(ctx)
	if err != nil {
		return nil, err
	}
	return &factoryservice.UninstallResponse{}, nil
}
