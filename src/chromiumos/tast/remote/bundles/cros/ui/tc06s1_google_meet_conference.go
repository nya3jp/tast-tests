// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/ui/pre"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC06S1GoogleMeetConference,
		Desc:         "Using Google Meet host a conference and presentation with participants",
		Contacts:     []string{"septemli@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		ServiceDeps: []string{
			"tast.cros.ui.GoogleMeetService",
			"tast.cros.cuj.LocalStoreService",
		},
		Pre:     pre.LocalStore(),
		Vars:    []string{"ui.cuj_username", "ui.cuj_password", "ui.meet_account", "ui.meet_url"},
		Timeout: time.Hour,
	})
}

func TC06S1GoogleMeetConference(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	tmpPath := s.PreValue().(*pre.LocalStoreData).DUTTempDir

	account := s.RequiredVar("ui.cuj_username")
	password := s.RequiredVar("ui.cuj_password")
	meetAccount := s.RequiredVar("ui.meet_account")
	meetURL := s.RequiredVar("ui.meet_url")

	u, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to dial to remote dut: ", err)
	}
	defer u.Close(ctx)

	ucli := pb.NewGoogleMeetServiceClient(u.Conn)

	defer func() {
		ereq := &pb.EndConferenceRequest{
			Account:  account,
			Password: password,
		}
		if _, err := ucli.EndConference(ctx, ereq); err != nil {
			s.Fatal("Failed to finish conference: ", err)
		}
	}()

	jreq := &pb.JoinMultipleParticipantsConferenceRequest{
		Account:     account,
		Password:    password,
		MeetAccount: meetAccount,
		Room:        meetURL,
	}
	if _, err := ucli.JoinMultipleParticipantsConference(ctx, jreq); err != nil {
		s.Fatal("Failed to join conference: ", err)
	}

	preq := &pb.PresentSlideRequest{
		Account:  account,
		Password: password,
		Room:     meetURL,
		TmpPath:  tmpPath,
	}
	if _, err := ucli.PresentSlide(ctx, preq); err != nil {
		s.Fatal("Failed to present slide: ", err)
	}

	sreq := &pb.StopPresentingRequest{
		Account:  account,
		Password: password,
	}
	if _, err := ucli.StopPresenting(ctx, sreq); err != nil {
		s.Fatal("Failed to stop presenting: ", err)
	}
}
