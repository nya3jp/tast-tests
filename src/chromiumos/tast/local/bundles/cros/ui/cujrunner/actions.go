// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cujrunner

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	actionRegistry = make(map[string]actionFunc)

	registerAction("OpenUrl", runActionOpenURL)

	registerAction("LockScreen", runActionLockScreen)
	registerAction("UnlockScreen", runActionUnlockScreen)

	registerAction("ClickUI", runActionClickUI)
}

// action holds information parsed from json config.
type action struct {
	Name  string           `json:"action"`
	Args  *json.RawMessage `json:"args,omitempty"`
	Start string           `json:"start,omitempty"`
}

// actionFunc defines an entry function type to run an action.
type actionFunc func(context.Context, *testing.State,
	*chrome.Chrome, *chrome.TestConn,
	*json.RawMessage) (func(context.Context) error, error)

var actionRegistry map[string]actionFunc

// registerAction associates an action name with its entry function.
func registerAction(n string, a actionFunc) {
	actionRegistry[n] = a
}

// getAction looks up the entry function of the action identified by the name.
func getAction(n string) (actionFunc, bool) {
	action, ok := actionRegistry[n]
	return action, ok
}

// actionArgsOpenURL defines args in json for runActionOpenURL.
type actionArgsOpenURL struct {
	URL string `json:"url"`
}

func runActionOpenURL(ctx context.Context, s *testing.State,
	cr *chrome.Chrome, tconn *chrome.TestConn,
	ad *json.RawMessage) (func(context.Context) error, error) {

	args := &actionArgsOpenURL{}
	if err := json.Unmarshal(*ad, args); err != nil {
		return nil, errors.Wrap(err, "failed to parse args")
	}

	conn, err := cr.NewConn(ctx, args.URL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open url")
	}

	return func(context.Context) error {
		return conn.Close()
	}, nil
}

func runActionLockScreen(ctx context.Context, s *testing.State,
	cr *chrome.Chrome, tconn *chrome.TestConn,
	ad *json.RawMessage) (func(context.Context) error, error) {
	return nil, lockscreen.Lock(ctx, tconn)
}

func runActionUnlockScreen(ctx context.Context, s *testing.State,
	cr *chrome.Chrome, tconn *chrome.TestConn,
	ad *json.RawMessage) (func(context.Context) error, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	password := s.RequiredVar("ui.cuj_password")
	if err := kb.Type(ctx, password+"\n"); err != nil {
		return nil, errors.Wrap(err, "failed to type password")
	}

	const goodAuthTimeout = 30 * time.Second
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, goodAuthTimeout); err != nil {
		return nil, errors.Wrapf(err, "failed to wait for screen unlock (last status %+v)", st)
	}

	return nil, nil
}

func runActionClickUI(ctx context.Context, s *testing.State,
	cr *chrome.Chrome, tconn *chrome.TestConn,
	ad *json.RawMessage) (func(context.Context) error, error) {
	args := &ui.FindParams{}
	if err := json.Unmarshal(*ad, args); err != nil {
		return nil, errors.Wrap(err, "failed to parse args")
	}

	return nil, ui.StableFindAndClick(ctx, tconn, *args,
		&testing.PollOptions{Timeout: 30 * time.Second})
}
