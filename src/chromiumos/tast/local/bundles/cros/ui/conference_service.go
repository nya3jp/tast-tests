// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/conference"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterConferenceServiceServer(srv, &ConferenceService{s: s})
		},
		Vars: []string{
			// mode is optional. Expecting "tablet" or "clamshell".
			"mode",
			// Chrome login credentials.
			"ui.cuj_username",
			"ui.cuj_password",
			// UI meet joining credentials.
			"ui.meet_account",
			"ui.meet_password",
			// Static Google meet room url.
			"ui.meet_url",
		},
	})
}

type ConferenceService struct {
	s *testing.ServiceState
}

func (s *ConferenceService) RunGoogleMeetScenario(ctx context.Context, req *pb.MeetScenarioRequest) (*empty.Empty, error) {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get outdir from context")
	}
	account, ok := s.s.Var("ui.cuj_username")
	if !ok {
		return nil, errors.New("failed to get variable ui.cuj_username")
	}
	password, ok := s.s.Var("ui.cuj_password")
	if !ok {
		return nil, errors.New("failed to get variable ui.cuj_password")
	}
	meetURL, ok := s.s.Var("ui.meet_url")
	if !ok {
		return nil, errors.New("failed to get variable ui.meet_url")
	}
	meetAccount, ok := s.s.Var("ui.meet_account")
	if !ok {
		return nil, errors.New("failed to get variable ui.meet_account")
	}
	meetPassword, ok := s.s.Var("ui.meet_password")
	if !ok {
		return nil, errors.New("failed to get variable ui.meet_password")
	}

	testing.ContextLog(ctx, "Start google meet scenario")
	cr, err := chrome.New(ctx,
		// Make sure we are running new chrome UI when tablet mode is enabled by CUJ test.
		// Remove this when new UI becomes default.
		chrome.EnableFeatures("WebUITabStrip"),
		chrome.KeepState(),
		chrome.ARCSupported(),
		chrome.GAIALogin(chrome.Creds{User: account, Pass: password}))
	if err != nil {
		return nil, err
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test API")
	}
	var tabletMode bool
	if mode, ok := s.s.Var("mode"); ok {
		tabletMode = mode == "tablet"
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to enable tablet mode to %v", tabletMode)
		}
		defer cleanup(ctx)
	} else {
		// Use default screen mode of the DUT.
		tabletMode, err = ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get DUT default screen mode")
		}
	}

	prepare := func(ctx context.Context) (string, conference.Cleanup, error) {
		// No need to cleanuup at the end of Google Meet conference.
		cleanup := func(ctx context.Context) (err error) {
			return nil
		}
		if meetURL == "" {
			return "", nil, errors.New("the conference invite link is empty")
		}
		return meetURL, cleanup, nil
	}

	// Creates a Google Meet conference instance which implements conference.Conference methods
	// which provides conference operations.
	gmcli := conference.NewGoogleMeetConference(cr, tconn, int(req.RoomSize), meetAccount, meetPassword)
	// Shorten context a bit to allow for cleanup if Run fails.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()
	if err := conference.Run(ctx, cr, gmcli, prepare, req.Tier, outDir, tabletMode, req.ExtendedDisplay); err != nil {
		return nil, errors.Wrap(err, "failed to run MeetConference")
	}

	return &empty.Empty{}, nil
}
