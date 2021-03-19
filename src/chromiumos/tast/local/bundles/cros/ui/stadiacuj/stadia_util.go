// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stadiacuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// StadiaGameURL is the url of the page of the testing game.
	StadiaGameURL = "https://ggp-staging.sandbox.google.com/store/details/af885a49666547028488e24465865765rct1/sku/37e10f659b9e41e69df224a94ad881ac"
	// StadiaGameName is the name of the testing game.
	StadiaGameName = "Mortal Kombat 11"
)

// PressKey presses a given key and waited for a given duration. Video games take time
// to process keyboard events so the intervals between two events are necessary.
func PressKey(ctx context.Context, kb *input.KeyboardEventWriter, s string, duration time.Duration) error {
	if err := kb.Accel(ctx, s); err != nil {
		return errors.Wrap(err, "failed to press the key")
	}
	if err := testing.Sleep(ctx, duration); err != nil {
		return errors.Wrap(err, "failed to wait")
	}
	return nil
}

// HoldKey holds a key for a given duration. Holding keys (especially arrow keys) is very common in
// video game playing.
func HoldKey(ctx context.Context, kb *input.KeyboardEventWriter, s string, duration time.Duration) error {
	if err := kb.AccelPress(ctx, s); err != nil {
		return errors.Wrap(err, "failed to long press the key")
	}
	if err := testing.Sleep(ctx, duration); err != nil {
		return errors.Wrap(err, "failed to wait")
	}
	if err := kb.AccelRelease(ctx, s); err != nil {
		return errors.Wrap(err, "failed to release the key")
	}
	return nil
}

// ExitGame holds esc key and exits the game.
func ExitGame(ctx context.Context, kb *input.KeyboardEventWriter, ac *uiauto.Context, webpage *nodewith.Finder) error {
	if err := HoldKey(ctx, kb, "esc", 2*time.Second); err != nil {
		return errors.Wrap(err, "failed to hold the sec key")
	}
	ac = ac.WithTimeout(10 * time.Second)
	exitButton := nodewith.Name("Exit game").Role(role.Button)
	return uiauto.Combine(
		"focus and click",
		ac.FocusAndWait(exitButton),
		ac.LeftClick(exitButton),
	)(ctx)
}
