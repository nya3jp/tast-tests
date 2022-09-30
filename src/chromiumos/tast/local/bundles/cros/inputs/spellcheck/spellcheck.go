// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spellcheck

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

const spellCheckPrefix = "spellCheckEnabled_"

// SetOneTimeMarkerWithWord sets a global variable to observe the spelling marker of the word changed.
func SetOneTimeMarkerWithWord(ctx context.Context, tconn *chrome.TestConn, word string) error {
	return tconn.Call(ctx, nil, `(name) => {
		// Set global variable.
		window[name] = false;

		let observer = chrome.automation.addTreeChangeObserver('textMarkerChanges', (treeChange) => {
			if (!treeChange.target.markers || treeChange.target.markers.length == 0) {
				return;
			}

			if (treeChange.target.markers[0].flags.spelling) {
				window[name] = true;
			}

			chrome.automation.removeTreeChangeObserver(observer);
		});
	}`, spellCheckPrefix+word)
}

// WaitUntilMarkerExists returns a function that waits for the spelling marker of the word exists.
func WaitUntilMarkerExists(tconn *chrome.TestConn, word string) uiauto.Action {
	enabled := false
	// The spelling marker has a delay, while the spellchecker process the word.
	// Poll will return a false error to give time to the marker to show.
	noSpellcheckErr := errors.New("spellCheckEnabled evaluated to false")
	return func(ctx context.Context) error {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := tconn.Eval(ctx, spellCheckPrefix+word, &enabled); err != nil {
				return testing.PollBreak(err)
			}
			if !enabled {
				return noSpellcheckErr
			}
			return nil
		}, &testing.PollOptions{Interval: 10 * time.Millisecond, Timeout: 15 * time.Second}); err != nil {
			if errors.Is(err, noSpellcheckErr) {
				return errors.Wrap(err, "spell check text marker is not enabled")
			}
			return errors.Wrap(err, "could not evaluate spellCheckEnabled")
		}
		return nil
	}
}
