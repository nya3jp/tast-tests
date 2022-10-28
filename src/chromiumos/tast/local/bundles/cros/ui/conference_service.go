// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/bond"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/conference"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/local/ui/cujrecorder"
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
			// Optional. Expecting "enable" or "disable", default is "disable".
			"ui.collectTrace",
			// CrOS login credentials.
			"ui.cujAccountPool",
			// Credentials used to join Google Meet. It might be different with CrOS login credentials.
			"ui.meet_account",
			"ui.meet_password",
			// Credentials for BOND API.
			"ui.GoogleMeetCUJ.bond_enabled",
			"ui.GoogleMeetCUJ.bond_key",

			// Static Google meet rooms with different participant number have been created.
			// They have different URLs. ui.meet_url can be used to run a specific subtest but
			// assigning urls to different vars will be easier when running with ui.GoogleMeetCUJ.*.
			// Each of the folliwng vars can be assigned with mutiple URLs, seperated by comma.
			// Test can retry another url if one fails.
			// - Primary URLs: use these URLs first.
			"ui.meet_url",
			"ui.meet_url_two",
			"ui.meet_url_small",
			"ui.meet_url_large",
			"ui.meet_url_class",
			// - Secondary URLs: only used when primary ones fail.
			"ui.meet_url_secondary",
			"ui.meet_url_two_secondary",
			"ui.meet_url_small_secondary",
			"ui.meet_url_large_secondary",
			"ui.meet_url_class_secondary",

			// The total timeout and inteval when trying different URLs if one fails.
			"ui.meet_url_retry_timeout",
			"ui.meet_url_retry_interval",
			// Zoom meet bot server address.
			"ui.zoom_bot_server",
			"ui.zoom_bot_token",
		},
	})
}

type ConferenceService struct {
	s *testing.ServiceState
}

func confereceChromeOpts(accountPool, cameraVideoPath string) []chrome.Option {
	chromeArgs := chromeArgsWithFileCameraInput(cameraVideoPath)
	return []chrome.Option{
		// Make sure we are running new chrome UI when tablet mode is enabled by CUJ test.
		// Remove this when new UI becomes default.
		chrome.EnableFeatures("WebUITabStrip"),
		chrome.KeepState(),
		chrome.ARCSupported(),
		chrome.GAIALoginPool(accountPool),
		chrome.ExtraArgs(chromeArgs...)}
}

// chromeArgsWithFileCameraInput returns Chrome extra args as string slice
// for video test with a Y4M/MJPEG fileName streamed as live camera input.
func chromeArgsWithFileCameraInput(fileName string) []string {
	if fileName == "" {
		return []string{}
	}
	return []string{
		// See https://webrtc.github.io/webrtc-org/testing/.
		// Feed a test pattern to getUserMedia() instead of live camera input.
		"--use-fake-device-for-media-stream",
		// Feed a Y4M/MJPEG test file to getUserMedia() instead of live camera input.
		"--use-file-for-fake-video-capture=" + fileName,
	}
}

// newConferenceChrome returns a new Chrome instance with custom options for confernce cuj,
// including setting whether to use fake camera and lacros browser.
func newConferenceChrome(ctx context.Context, accountPool, cameraVideoPath string, bt browser.Type) (cr *chrome.Chrome, err error) {
	opts := confereceChromeOpts(accountPool, cameraVideoPath)
	lacrosCfg := lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(chrome.LacrosEnableFeatures("WebUITabStrip")))
	cr, err = browserfixt.NewChrome(ctx, bt, lacrosCfg, opts...)
	if err != nil {
		return cr, errors.Wrap(err, "failed to restart Chrome")
	}
	preTest(ctx)
	return cr, nil
}

func preTest(ctx context.Context) {
	// Wait for cpu to idle before test.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		// Log the cpu idle wait failure instead of make it fatal.
		testing.ContextLog(ctx, "Failed to wait for CPU to become idle: ", err)
	}
}

const tmpDir = "/tmp"

func (s *ConferenceService) RunGoogleMeetScenario(ctx context.Context, req *pb.MeetScenarioRequest) (*empty.Empty, error) {
	roomType := conference.RoomType(req.RoomType)
	isNoRoom := roomType == conference.NoRoom
	meet, err := conference.GetGoogleMeetConfig(ctx, s.s, roomType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get meet config")
	}
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get outdir from context")
	}
	traceConfigPath := ""
	if collect, ok := s.s.Var("ui.collectTrace"); ok && collect == "enable" {
		traceConfigPath = tmpDir + "/" + cujrecorder.SystemTraceConfigFile
	}
	run := func(ctx context.Context, roomURL string) error {
		accountPool, ok := s.s.Var("ui.cujAccountPool")
		if !ok {
			return errors.New("failed to get variable ui.cujAccountPool")
		}
		bt := browser.TypeAsh
		if req.IsLacros {
			bt = browser.TypeLacros
		}
		cr, err := newConferenceChrome(ctx, accountPool, req.CameraVideoPath, bt)
		if err != nil {
			return errors.Wrap(err, "failed to new Chrome")
		}
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to connect to test API")
		}
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to initialize keyboard input")
		}
		defer kb.Close()
		var tabletMode bool
		cleanupCtx := ctx
		ctx, cancelTablet := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancelTablet()
		if mode, ok := s.s.Var("ui.cuj_mode"); ok {
			tabletMode = mode == "tablet"
			cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
			if err != nil {
				return errors.Wrapf(err, "failed to enable tablet mode to %v", tabletMode)
			}
			defer cleanup(cleanupCtx)
		} else {
			// Use default screen mode of the DUT.
			tabletMode, err = ash.TabletModeEnabled(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get DUT default screen mode")
			}
		}
		testing.ContextLog(ctx, "Running test with tablet mode: ", tabletMode)
		var uiHandler cuj.UIActionHandler
		if tabletMode {
			cleanup, err := display.RotateToLandscape(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to rotate display to landscape")
			}
			defer cleanup(cleanupCtx)
			if uiHandler, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to create tablet action handler")
			}
		} else {
			if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to create clamshell action handler")
			}
		}

		if req.ExtendedDisplay {
			// Unset mirrored display so two displays can show different information.
			if err := typecutils.SetMirrorDisplay(ctx, tconn, false); err != nil {
				return errors.Wrap(err, "failed to unset mirror display")
			}
			// Make sure there are two displays on DUT.
			// This procedure must be performed after display mirror is unset. Otherwise we can only
			// get one display info.
			infos, err := display.GetInfo(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get display info")
			}

			if len(infos) != 2 {
				return errors.Errorf("expect 2 displays but got %d", len(infos))
			}
		}

		prepare := func(ctx context.Context) (string, conference.Cleanup, error) {
			cleanup := func(ctx context.Context) (err error) {
				// Nothing to clean up at the end of Google Meet conference.
				return nil
			}
			if !isNoRoom && roomURL == "" {
				return "", nil, errors.New("the conference invite link is empty")
			}
			return roomURL, cleanup, nil
		}

		// Creates a Google Meet conference instance which implements conference.Conference methods
		// which provides conference operations.
		gmcli := conference.NewGoogleMeetConference(cr, tconn, kb, uiHandler, tabletMode, req.ExtendedDisplay, bt, roomType, meet.Account, meet.Password, outDir)
		defer gmcli.End(cleanupCtx)
		// Shorten context a bit to allow for cleanup if Run fails.
		ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
		defer cancel()

		if err := conference.Run(ctx, cr, gmcli, prepare, req.Tier, outDir, traceConfigPath, tabletMode, bt, roomType); err != nil {
			return errors.Wrap(err, "failed to run Google Meet conference")
		}
		return nil
	}
	if isNoRoom {
		// Without Google Meet, there is no need to assign a meet url.
		if err := run(ctx, ""); err != nil {
			testing.ContextLogf(ctx, "Failed to run conference: %+v", err)
			return nil, err
		}
		return &empty.Empty{}, nil
	}

	if meet.BondEnabled {
		for { // Using for loop for the sake of breaking out of the block.
			// Connect.
			bondConn, err := bond.NewClient(ctx, bond.WithCredsJSON(meet.BondCreds), bond.WithExternalEndpoint())
			if err != nil {
				testing.ContextLogf(ctx, "BOND API2: Failed to connect: %+v", err)
				break
			}
			defer bondConn.Close()

			// Create room with bots.
			botsDuration := 60 * time.Minute // one hour long by default.
			deadline, ok := ctx.Deadline()
			if ok {
				botsDuration = deadline.Add(90 * time.Second).Sub(time.Now())
			}
			numBots := conference.GoogleMeetRoomParticipants[roomType] - 1 // one of participants is the test itself
			bondMeetingCode, numFailures, err := bondConn.CreateConferenceWithBots(ctx, numBots, botsDuration)
			defer bondConn.RemoveAllBotsFromConference(ctx, bondMeetingCode)
			if err != nil || numFailures > 0 {
				testing.ContextLogf(ctx, "BOND API2: %d bots failed to connect. Error: %+v", numFailures, err)
				break
			}
			testing.ContextLogf(ctx, "BOND API2: Created conference: %+v and added %d bots for the duration of %v", bondMeetingCode, numBots, botsDuration)

			// Make the room created by BOND the first one to try.
			meet.URLs = append([]string{fmt.Sprintf("https://meet.google.com/%s", bondMeetingCode)}, meet.URLs...)

			break
		}

		if len(meet.URLs) == 0 {
			return nil, errors.Wrap(err, "failed to create a meeting via BOND API and no static rooms specified")
		}
	}

	runWithMeetUrls := func(ctx context.Context) error {
		var err error
		for _, url := range meet.URLs {
			testing.ContextLog(ctx, "URL to be tested in the meet url list: ", url)
			err = run(ctx, url)
			if err == nil {
				return nil
			}
			if !conference.IsParticipantError(err) {
				return err
			}
		}
		return err
	}
	// If meet.RetryTimeout equal to 0, don't do any retry.
	if meet.RetryTimeout == 0 {
		testing.ContextLog(ctx, "Start running meet scenario")
		if err := runWithMeetUrls(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to run conference: %+v", err) // Print error with stack trace.
			return nil, err
		}
		return &empty.Empty{}, nil
	}

	var lastError error
	startTime := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := runWithMeetUrls(ctx); err != nil {
			elapsedTime := time.Now().Sub(startTime)
			if elapsedTime < meet.RetryTimeout {
				// Record the complete run result if the failure is not because of timeout.
				lastError = err
			}
			if conference.IsParticipantError(err) {
				testing.ContextLogf(ctx, "Wait %v and try to run meet scenario again; caused by error: %v", meet.RetryInterval, err)
				return err
			}
			return testing.PollBreak(err) // Break if error is not participant number related.
		}
		return nil
	}, &testing.PollOptions{Timeout: meet.RetryTimeout, Interval: meet.RetryInterval}); err != nil {
		// Return test failure reason of last complete run.
		if lastError != nil {
			err = lastError
		}
		testing.ContextLogf(ctx, "Failed to run conference: %+v", err) // Print error with stack trace.
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *ConferenceService) RunZoomScenario(ctx context.Context, req *pb.MeetScenarioRequest) (*empty.Empty, error) {
	type responseData struct {
		URL    string `json:"url"`
		RoomID string `json:"room_id"`
		Err    string `json:"err"`
	}
	roomType := conference.RoomType(req.RoomType)
	runConferenceAPI := func(ctx context.Context, sessionToken, host, api, parameterString string) (*responseData, error) {
		reqURL := fmt.Sprintf("%s/api/room/zoom/%s%s&iszoomcase=true", host, api, parameterString)
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
	accountPool, ok := s.s.Var("ui.cujAccountPool")
	if !ok {
		return nil, errors.New("failed to get variable ui.cujAccountPool")
	}
	host, ok := s.s.Var("ui.zoom_bot_server")
	if !ok {
		return nil, errors.New("failed to get variable ui.zoom_bot_server")
	}

	sessionToken, ok := s.s.Var("ui.zoom_bot_token")
	if !ok {
		return nil, errors.New("failed to get variable ui.zoom_bot_token")
	}
	traceConfigPath := ""
	if collect, ok := s.s.Var("ui.collectTrace"); ok && collect == "enable" {
		traceConfigPath = tmpDir + "/" + cujrecorder.SystemTraceConfigFile
	}

	testing.ContextLog(ctx, "Start zoom meet scenario")
	bt := browser.TypeAsh
	if req.IsLacros {
		bt = browser.TypeLacros
	}
	cr, err := newConferenceChrome(ctx, accountPool, req.CameraVideoPath, bt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to new Chrome")
	}
	account := cr.Creds().User

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test API")
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()
	var tabletMode bool
	cleanupCtx := ctx
	ctx, cancelTablet := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancelTablet()
	if mode, ok := s.s.Var("ui.cuj_mode"); ok {
		tabletMode = mode == "tablet"
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to enable tablet mode to %v", tabletMode)
		}
		defer cleanup(cleanupCtx)
	} else {
		// Use default screen mode of the DUT.
		tabletMode, err = ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get DUT default screen mode")
		}
	}

	var uiHandler cuj.UIActionHandler
	if tabletMode {
		cleanup, err := display.RotateToLandscape(ctx, tconn)
		if err != nil {
			return nil, errors.Wrap(err, "failed to rotate display to landscape")
		}
		defer cleanup(cleanupCtx)
		if uiHandler, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			return nil, errors.Wrap(err, "failed to create tablet action handler")
		}
	} else {
		if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			return nil, errors.Wrap(err, "failed to create clamshell action handler")
		}
	}
	// Creates a Zoom conference instance which implements conference.Conference methods.
	// which provides conference operations.
	zmcli := conference.NewZoomConference(cr, tconn, kb, uiHandler, tabletMode, roomType, account, outDir)
	defer zmcli.End(cleanupCtx)
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
		var data *responseData
		roomSize := conference.ZoomRoomParticipants[roomType] - 1
		// Create a Zoom conference on remote server dynamically and get conference room
		// link. Retry three times until it successfully gets a conference room link.
		const retryCount = 3
		for i := 0; i < retryCount; i++ {
			testing.ContextLogf(ctx, "Attempt #%d to get conference room API", i+1)
			// Use the remaining time of the case to set the existence time of the room.
			deadline, _ := ctx.Deadline()
			maxDuration := math.Ceil(deadline.Sub(time.Now()).Minutes())
			parameterString := fmt.Sprintf("?count=%d&max_duration=%v", roomSize, maxDuration)
			testing.ContextLogf(ctx, "Create a %d-person zoom room that can exist for %v minutes", roomSize, maxDuration)
			if data, err = runConferenceAPI(ctx, sessionToken, host, "createaio", parameterString); err == nil {
				break
			}
			testing.ContextLog(ctx, "Failed to get conference room: ", err)
		}
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
	if err := conference.Run(ctx, cr, zmcli, prepare, req.Tier, outDir, traceConfigPath, tabletMode, bt, roomType); err != nil {
		return nil, errors.Wrap(err, "failed to run Zoom conference")
	}

	return &empty.Empty{}, nil
}
