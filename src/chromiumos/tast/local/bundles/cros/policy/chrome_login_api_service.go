// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/session"
	pb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterChromeLoginAPIServiceServer(srv, &ChromeLoginAPIService{s: s})
		},
	})
}

// ChromeLoginAPIService implements tast.cros.policy.ChromeLoginAPIService.
type ChromeLoginAPIService struct { // NOLINT
	s *testing.ServiceState
}

// TestLaunchManagedGuestSession uses the provided extension ID to to launch a MGS using
// chrome.login.launchManagedGuestSession and checks that the session was launched.
func (c *ChromeLoginAPIService) TestLaunchManagedGuestSession(ctx context.Context, req *pb.TestLaunchManagedGuestSessionRequest) (*empty.Empty, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.KeepState(),
	)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to connect to session_manager")
	}

	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to watch for D-Bus signals")
	}
	defer sw.Close(ctx)

	bgURL := chrome.ExtensionBackgroundPageURL(req.ExtensionID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to connect to background page")
	}
	defer conn.Close()

	if err := conn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		chrome.login.launchManagedGuestSession(() => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
			}
			resolve();
		});
	})`, nil); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to launch managed guest session")
	}

	select {
	case <-sw.Signals:
		return &empty.Empty{}, nil
	case <-ctx.Done():
		return &empty.Empty{}, errors.Wrap(err, "didn't get SessionStateChanged signal")
	}
}
