// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/conference"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
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
			"ui.cuj_mode",
			// CrOS login credentials.
			"ui.cuj_username",
			"ui.cuj_password",
			// Credentials used to join Google Meet. It might be different with CrOS login credentials.
			"ui.meet_account",
			"ui.meet_password",
			// Static Google meet rooms with different participant number have been created.
			// They have different URLs. ui.meet_url can be used to run a specific subtest but
			// assigning urls to different vars will be easier when running with ui.GoogleMeetCUJ.*.
			"ui.meet_url",
			"ui.meet_url_two",
			"ui.meet_url_small",
			"ui.meet_url_large",
			"ui.meet_url_class",
			// Zoom meet bot server address.
			"ui.zoom_bot_server",
			"ui.zoom_bot_token",
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
	// Convert to lower case because user account is case-insensitive and shown as lower case in CrOS.
	account = strings.ToLower(account)
	password, ok := s.s.Var("ui.cuj_password")
	if !ok {
		return nil, errors.New("failed to get variable ui.cuj_password")
	}
	meetAccount, ok := s.s.Var("ui.meet_account")
	if !ok {
		return nil, errors.New("failed to get variable ui.meet_account")
	}
	// Convert to lower case because user account is case-insensitive and shown as lower case in CrOS.
	meetAccount = strings.ToLower(meetAccount)
	meetPassword, ok := s.s.Var("ui.meet_password")
	if !ok {
		return nil, errors.New("failed to get variable ui.meet_password")
	}

	var urlVar string
	switch req.RoomSize {
	case conference.SmallRoomSize:
		urlVar = "ui.meet_url_small"
	case conference.LargeRoomSize:
		urlVar = "ui.meet_url_large"
	case conference.ClassRoomSize:
		urlVar = "ui.meet_url_class"
	default:
		urlVar = "ui.meet_url_two"
	}
	meetURL, ok := s.s.Var(urlVar)
	if !ok {
		// if specific meeting url is not found, try the general meet url var.
		if meetURL, ok = s.s.Var("ui.meet_url"); !ok {
			return nil, errors.Errorf("failed to get variable ui.meet_url or %s", urlVar)
		}
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
	if mode, ok := s.s.Var("ui.cuj_mode"); ok {
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
	testing.ContextLog(ctx, "Running test with tablet mode: ", tabletMode)

	if req.ExtendedDisplay {
		// Unset mirrored display so two displays can show different information.
		if err := cuj.UnsetMirrorDisplay(ctx, tconn); err != nil {
			return nil, errors.Wrap(err, "failed to unset mirror display")
		}
		// Make sure there are two displays on DUT.
		// This procedure must be performed after display mirror is unset. Otherwise we can only
		// get one display info.
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get display info")
		}

		if len(infos) != 2 {
			return nil, errors.Errorf("expect 2 displays but got %d", len(infos))
		}
	}

	prepare := func(ctx context.Context) (string, conference.Cleanup, error) {
		cleanup := func(ctx context.Context) (err error) {
			// Nothing to clean up at the end of Google Meet conference.
			return nil
		}
		if meetURL == "" {
			return "", nil, errors.New("the conference invite link is empty")
		}
		return meetURL, cleanup, nil
	}

	// Creates a Google Meet conference instance which implements conference.Conference methods
	// which provides conference operations.
	gmcli := conference.NewGoogleMeetConference(cr, tconn, tabletMode, int(req.RoomSize), meetAccount, meetPassword)
	defer gmcli.End(ctx)
	// Shorten context a bit to allow for cleanup if Run fails.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	if err := conference.Run(ctx, cr, gmcli, prepare, req.Tier, outDir, tabletMode, req.ExtendedDisplay); err != nil {
		testing.ContextLogf(ctx, "Failed to run conference: %+v", err) // Print error with stack trace.

		// Dump the UI tree to the service/faillog subdirectory.
		// Don't dump directly into outDir
		// because it might be overridden by the test faillog after pulled back to remote server.
		faillog.DumpUITreeWithScreenshotOnError(ctx, filepath.Join(outDir, "service"), func() bool { return true }, cr, "ui_dump")
		return nil, errors.Wrap(err, "failed to run MeetConference")
	}

	return &empty.Empty{}, nil
}

func (s *ConferenceService) RunZoomScenario(ctx context.Context, req *pb.MeetScenarioRequest) (*empty.Empty, error) {
	type responseData struct {
		URL    string `json:"url"`
		RoomID string `json:"room_id"`
		Err    string `json:"err"`
	}

	runConferenceAPI := func(ctx context.Context, sessionToken, host, api, parameterData string) (*responseData, error) {
		reqURL := fmt.Sprintf("%s/api/room/zoom/%s%s", host, api, parameterData)
		testing.ContextLog(ctx, "Requesting a zoom room from the zoom bot server with request URL: ", reqURL)
		httpReq, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+sessionToken)
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, errors.Errorf("failed to get zoom conference invite link with status %d and body %s", resp.StatusCode, body)
		}

		var data *responseData
		if err := json.Unmarshal([]byte(body), &data); err != nil {
			return nil, err
		}
		if data.Err != "" {
			return data, errors.New(data.Err)
		}
		return data, nil
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get outdir from context")
	}
	account, ok := s.s.Var("ui.cuj_username")
	if !ok {
		return nil, errors.New("failed to get variable ui.cuj_username")
	}
	// user account is case-insensitive and shown as lower case in CrOS.
	account = strings.ToLower(account)
	password, ok := s.s.Var("ui.cuj_password")
	if !ok {
		return nil, errors.New("failed to get variable ui.cuj_password")
	}
	host, ok := s.s.Var("ui.zoom_bot_server")
	if !ok {
		return nil, errors.New("failed to get variable ui.zoom_bot_server")
	}

	sessionToken, ok := s.s.Var("ui.zoom_bot_token")
	if !ok {
		return nil, errors.New("failed to get variable ui.zoom_bot_token")
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
	if mode, ok := s.s.Var("ui.cuj_mode"); ok {
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

	var tsAction cuj.UIActionHandler
	if tabletMode {
		if tsAction, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			return nil, errors.Wrap(err, "failed to create tablet action handler")
		}
	} else {
		if tsAction, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			return nil, errors.Wrap(err, "failed to create clamshell action handler")
		}
	}
	// Creates a Zoom conference instance which implements conference.Conference methods.
	// which provides conference operations.
	zmcli := conference.NewZoomConference(cr, tconn, tsAction, tabletMode, int(req.RoomSize), account)
	defer zmcli.End(ctx)
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
		// Creates a Zoom conference on remote server dynamically and get
		// conference room link.
		roomSize := strconv.FormatInt(req.RoomSize-1, 10)
		data, err := runConferenceAPI(ctx, sessionToken, host, "createaio", "?count="+roomSize)
		if err != nil {
			return "", nil, errors.Wrap(err, "failed to create multiple participants room")
		}

		// We expect the returned body is a valid url that can be used to issue chatroom request.
		// Check the format.
		room := strings.TrimSpace(string(data.URL))
		if _, err := url.ParseRequestURI(room); err != nil {
			return "", nil, errors.Errorf("returned zoom conference invite link %s is not a valid url", room)
		}

		cleanup := func(ctx context.Context) (err error) {
			_, err = runConferenceAPI(ctx, sessionToken, host, "endaio", "?room_id="+data.RoomID)
			return
		}

		return room, cleanup, nil
	}
	// Shorten context a bit to allow for cleanup if Run fails.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()
	if err := conference.Run(ctx, cr, zmcli, prepare, req.Tier, outDir, tabletMode, req.ExtendedDisplay); err != nil {
		testing.ContextLogf(ctx, "Failed to run conference: %+v", err) // Print error with stack trace.

		// Dump the UI tree to the service/faillog subdirectory.
		// Don't dump directly into outDir
		// because it might be overridden by the test faillog after pulled back to remote server.
		faillog.DumpUITreeWithScreenshotOnError(ctx, filepath.Join(outDir, "service"), func() bool { return true }, cr, "ui_dump")
		return nil, errors.Wrap(err, "failed to run MeetConference")
	}

	return &empty.Empty{}, nil
}
