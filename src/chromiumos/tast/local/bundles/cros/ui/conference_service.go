// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
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
			// Zoom meet bot server address.
			"ui.conference_server",
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

func (s *ConferenceService) RunZoomScenario(ctx context.Context, req *pb.MeetScenarioRequest) (*empty.Empty, error) {
	runConferenceAPI := func(host, api string) (string, error) {
		reqURL := fmt.Sprintf("http://%s/api/room/zoom/%s", host, api)
		httpClient := &http.Client{Timeout: time.Minute * 10}
		resp, err := httpClient.Get(reqURL)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		if resp.StatusCode != http.StatusOK {
			return "", errors.Wrap(errors.New(string(body)), "failed to get zoom conference invite link")
		}

		if _, err := url.ParseRequestURI(strings.TrimSpace(string(body))); err != nil {
			return "", err
		}

		return string(body), nil
	}

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
	host, ok := s.s.Var("ui.conference_server")
	if !ok {
		return nil, errors.New("failed to get variable conference_server")
	}

	testing.ContextLog(ctx, "Start zoom meet scenario")
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
	// Creates a Zoom conference instance which implements conference.Conference methods.
	// which provides conference operations.
	zmcli := conference.NewZoomConference(cr, tconn, account)

	// Sends a http request that ask for creating a Zoom conferece with
	// specified participants and also return clean up method for closing
	// opened conference.
	//
	// Assume there's a Zoom proxy which can receive http request for
	// creating/closing Zoom conference. When Zoom proxy receives "createaio"
	// request, it would create a Zoom conference on specified remote server
	// with participants via Chrome Devtools Protocols. And "endaio" means close
	// the conference which opened by "createaio".
	prepare := func(ctx context.Context) (string, conference.Cleanup, error) {
		cleanup := func(ctx context.Context) (err error) {
			_, err = runConferenceAPI(host, "endaio")
			return
		}
		// Creates a Zoom conference on remote server dynamically and get
		// conference room link.
		room, err := runConferenceAPI(host, "createaio")
		if err != nil {
			return "", nil, errors.Wrap(err, "failed to create multiple participants room")
		}
		return room, cleanup, nil
	}
	// Shorten context a bit to allow for cleanup if Run fails.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()
	if err := conference.Run(ctx, cr, zmcli, prepare, req.Tier, outDir, tabletMode, req.ExtendedDisplay); err != nil {
		return nil, errors.Wrap(err, "failed to run MeetConference")
	}

	return &empty.Empty{}, nil
}
