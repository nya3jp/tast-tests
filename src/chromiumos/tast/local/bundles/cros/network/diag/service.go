// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diag

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/conndiag"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			network.RegisterNetDiagServiceServer(srv, &NetDiagService{s: s})
		},
	})
}

// NetDiagService implements tast.cros.network.DiagService
type NetDiagService struct {
	s *testing.ServiceState

	cr   *chrome.Chrome
	conn *chrome.Conn
	app  *conndiag.App
	api  *MojoAPI
}

// SetupDiagAPI creates a new chrome instance and launches the connectivity
// diagnostics application to be used for running the network diagnostics.
func (d *NetDiagService) SetupDiagAPI(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if d.api != nil {
		return nil, errors.New("Network diagnostics API is already setup")
	}

	success := false

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	defer func() {
		if !success {
			cr.Close(ctx)
		}
	}()

	app, err := conndiag.Launch(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch connectivity diagnostics app")
	}

	conn, err := app.ChromeConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get network diagnostics connection")
	}
	defer func() {
		if !success {
			conn.Close()
		}
	}()

	api, err := NewMojoAPI(ctx, conn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get network diagnostics mojo API")
	}
	defer func() {
		if !success {
			api.Release(ctx)
		}
	}()

	success = true
	d.cr = cr
	d.conn = conn
	d.api = api
	return &empty.Empty{}, nil
}

// Close will close the connectivity diagnostics application and the
// underlying Chrome instance.
func (d *NetDiagService) Close(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if err := d.api.Release(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to release Network Diagnostics mojo API: ", err)
	}
	if err := d.conn.Close(); err != nil {
		testing.ContextLog(ctx, "Failed to close Chrome connection to app: ", err)
	}
	if err := d.cr.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close Chrome connection: ", err)
	}

	return &empty.Empty{}, nil
}

// RunRoutine will run the specified network diagnostic routine and return the
// result.
func (d *NetDiagService) RunRoutine(ctx context.Context, req *network.RunRoutineRequest) (*network.RoutineResult, error) {
	if d.api == nil {
		return nil, errors.New("Network Diagnostics API has not been setup")
	}

	res, err := d.api.RunRoutine(ctx, req.Routine)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run routine")
	}

	return &network.RoutineResult{Verdict: int32(res.Verdict), Problems: res.Problems}, nil
}
