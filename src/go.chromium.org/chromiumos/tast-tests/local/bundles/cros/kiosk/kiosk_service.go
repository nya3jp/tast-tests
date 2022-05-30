// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/tast/errors"
	"go.chromium.org/chromiumos/tast-tests/local/kioskmode"
	"go.chromium.org/chromiumos/tast-tests/local/syslog"
	ppb "go.chromium.org/chromiumos/tast-tests/services/cros/kiosk"
	"go.chromium.org/chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ppb.RegisterKioskServiceServer(srv, &KioskService{s: s})
		},
	})
}

// KioskService implements tast.cros.kiosk.KioskService.
type KioskService struct { // NOLINT
	s *testing.ServiceState
}

// ConfirmKioskStarted confirms kiosk mode started.
func (c *KioskService) ConfirmKioskStarted(ctx context.Context, req *ppb.ConfirmKioskStartedRequest) (*empty.Empty, error) {
	reader, err := syslog.NewReader(ctx, syslog.Program(syslog.Chrome))
	if err != nil {
		return nil, errors.Wrap(err, "failed to run NewReader")
	}
	defer reader.Close()

	if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
		return nil, errors.Wrap(err, "There was a problem while checking chrome logs for Kiosk related entries")
	}

	return &empty.Empty{}, nil
}
