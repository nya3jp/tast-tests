// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/conference"
	"chromiumos/tast/local/bundles/cros/ui/conference/googlemeet"
	"chromiumos/tast/local/chrome"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterGoogleMeetServiceServer(srv, &GoogleMeetService{s})
		},
		Vars: []string{
			"ui.meet_account",
			"ui.meet_password",
			"ui.meet_url",
		},
	})
}

type GoogleMeetService struct {
	*testing.ServiceState
}

func (s *GoogleMeetService) MeetScenario(ctx context.Context, req *pb.MeetScenarioRequest) (*empty.Empty, error) {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return &empty.Empty{}, errors.Wrap(nil, "failed to get outdir from context")
	}
	testing.ContextLog(ctx, "Start google meet scenario")
	cr, err := chrome.New(ctx, chrome.KeepState(), chrome.ARCSupported(), chrome.GAIALogin(chrome.Creds{User: req.Account, Pass: req.Password}))
	if err != nil {
		return &empty.Empty{}, err
	}

	meetURL, ok := s.Var("ui.meet_url")
	if !ok {
		return &empty.Empty{}, errors.New("failed to get variable: ui.meet_url")
	}
	meetAccount, ok := s.Var("ui.meet_account")
	if !ok {
		return &empty.Empty{}, errors.New("failed to get variable: ui.meet_account")
	}
	meetPassword, ok := s.Var("ui.meet_password")
	if !ok {
		return &empty.Empty{}, errors.New("failed to get variable: ui.meet_password")
	}

	prepare := func() (string, conference.Cleanup, error) {
		return meetURL, nil, nil
	}

	// Creates a Google Meet conference instance which implements conference.Conference methods
	// which provides conference operations.
	gmcli := googlemeet.NewGoogleMeetConference(cr, int(req.RoomSize), meetAccount, meetPassword)
	if err := conference.MeetConference(ctx, cr, gmcli, prepare, req.Tier, outDir, req.TabletMode, req.ExtendedDisplay); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to run MeetConference")
	}

	return &empty.Empty{}, nil
}
