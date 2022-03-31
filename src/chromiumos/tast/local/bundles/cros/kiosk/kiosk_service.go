// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/syslog"
	ppb "chromiumos/tast/services/cros/kiosk"
	"chromiumos/tast/testing"
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
	//testing.ContextLogf(ctx, "Enrolling using Chrome with username: %s, dmserver: %s", string(req.Username), string(req.DmserverURL))

	//cr, err := chrome.New(
	//	ctx,
	//	chrome.GAIAEnterpriseEnroll(chrome.Creds{User: req.Username, Pass: req.Password}),
	//	chrome.GAIALogin(chrome.Creds{User: req.Username, Pass: req.Password}),
	//	chrome.DMSPolicy(req.DmserverURL),
	//)
	reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to run NewReader")
	}
	defer reader.Close()

	if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
		return nil, errors.Wrap(err, "There was a problem while checking chrome logs for Kiosk related entries")
	}

	//c.chrome = cr

	return &empty.Empty{}, nil
}
