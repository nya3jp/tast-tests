// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

const (
	// NoRoom means not joining google meet when running the test.
	NoRoom = 0
	// TwoRoomSize creates a conference room with 2 participants.
	TwoRoomSize = 2
	// SmallRoomSize creates a conference room with 5 participants.
	SmallRoomSize = 5
	// LargeRoomSize creates a conference room with 16 participants.
	LargeRoomSize = 16
	// ClassRoomSize creates a conference room with 38 participants.
	ClassRoomSize = 38
)

const (
	longUITimeout   = time.Minute      // Used for situations where UI might take a long time to respond.
	mediumUITimeout = 30 * time.Second // Used for situations where UI response are slower.
	shortUITimeout  = 3 * time.Second  // Used for situations where UI response are faster.
	viewingTime     = 5 * time.Second  // Used to view the effect after clicking application.
)

// Conference contains user's operation when enter a confernece room.
type Conference interface {
	Join(ctx context.Context, room string, toBlur bool) error
	SetLayoutMax(ctx context.Context) error
	SetLayoutMin(ctx context.Context) error
	SwitchTabs(ctx context.Context) error
	VideoAudioControl(ctx context.Context) error
	TypingInChat(ctx context.Context) error
	BackgroundChange(ctx context.Context) error
	Presenting(ctx context.Context, application googleApplication) error
	End(ctx context.Context) error
	SetBrowser(br *browser.Browser)
	LostNetworkCount() int
	DisplayAllParticipantsTime() time.Duration
}

const participantError = "number of participants is incorrect (ERROR - PARTICIPANT NUMBER)"
const signedOutError = "the account has been signed out: "

// ParticipantError wraps the given error with participant error specific information
// which can be used to identify the error type with IsParticipantError() function.
func ParticipantError(err error) error {
	return errors.Wrap(err, participantError)
}

// IsParticipantError returns true if the given error contains participant error specific information.
func IsParticipantError(err error) bool {
	// Use string comparason because error loses its type after wrapping.
	return strings.Contains(err.Error(), participantError)
}

// CheckSignedOutError check whether the account is signed out or not.
// If the acount is signed out, wraps the given error with signed out error specific information.
// If any other error happens or there is no signed out message, the original error will be returned.
func CheckSignedOutError(ctx context.Context, tconn *chrome.TestConn, err error) error {
	ui := uiauto.New(tconn)
	signedOutMessage := nodewith.NameRegex(regexp.MustCompile("(Sign in to add a Google account|You have been signed out).*")).First()
	// If the signed out message doesn't exist, ui.Info will wait 15s.
	// So first use ui.Exists to immediately check if there is a signed out message.
	if existsErr := ui.Exists(signedOutMessage)(ctx); existsErr != nil {
		return err
	}
	info, infoErr := ui.Info(ctx, signedOutMessage)
	if infoErr != nil {
		return err
	}
	return errors.Wrap(err, signedOutError+info.Name)
}
