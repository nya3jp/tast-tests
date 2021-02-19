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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/conference"
	"chromiumos/tast/local/bundles/cros/ui/conference/zoom"
	"chromiumos/tast/local/chrome"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterZoomServiceServer(srv, &ZoomService{s})
		},
		Vars: []string{"ui.conference_server"},
	})
}

type ZoomService struct {
	*testing.ServiceState
}

func (s *ZoomService) MeetScenario(ctx context.Context, req *pb.MeetScenarioRequest) (*empty.Empty, error) {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return &empty.Empty{}, errors.Wrap(nil, "failed to get outdir from context")
	}
	testing.ContextLog(ctx, "Start zoom meet scenario")
	cr, err := chrome.New(ctx, chrome.KeepState(), chrome.ARCSupported(), chrome.GAIALogin(), chrome.Auth(req.Account, req.Password, "gaia-id"))
	if err != nil {
		return &empty.Empty{}, err
	}

	host, ok := s.Var("ui.conference_server")
	if !ok {
		return &empty.Empty{}, errors.New("failed to get variable: conference_server")
	}

	// create Zoom Client
	zmcli := zoom.NewZoomConference(cr, req.Account)
	conferenceName := "zoom"
	prepare := func() (string, error) {
		room, err := runConferenceAPI(host, conferenceName, "createaio", "")
		if err != nil {
			return "", errors.Wrap(err, "failed to create multiple participants room")
		}
		return room, nil
	}
	cleanup := func() {
		runConferenceAPI(host, conferenceName, "endaio", "")
	}
	if err := conference.MeetConference(ctx, cr, zmcli, prepare, cleanup, req.Tier, outDir, req.TabletMode); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to run MeetConference")
	}

	return &empty.Empty{}, nil
}

// runConferenceAPI run conference api.
func runConferenceAPI(host, conferenceName, api, room string) (string, error) {
	reqURL := fmt.Sprintf("http://%s/api/room/%s/%s", host, conferenceName, api)
	if room != "" {
		reqURL = reqURL + "?url=" + room
	}
	httpClient := &http.Client{Timeout: time.Minute * 6}
	resp, err := httpClient.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.Wrap(nil, "failed to get zoom conference invite link")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if _, err := url.ParseRequestURI(strings.TrimSpace(string(body))); err != nil {
		return "", err
	}

	return string(body), nil
}
