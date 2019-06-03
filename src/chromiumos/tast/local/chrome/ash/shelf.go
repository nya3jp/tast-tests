// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ash implements a library used for communication with Chrome Ash.
package ash

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// ShelfBehavior represents the different Chrome OS shelf behaviors.
type ShelfBehavior string

// As defined in ShelfAutoHideBehavior here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/shelf_types.h
const (
	// ShelfBehaviorAlwaysAutoHide represents always auto-hide.
	ShelfBehaviorAlwaysAutoHide ShelfBehavior = "always"
	//ShelfBehaviorNeverAutoHide represents never auto-hide, meaning that it is always visible.
	ShelfBehaviorNeverAutoHide = "never"
	// ShelfBehaviorHidden represents always hidden, used for debugging, since this state is not exposed to the user.
	ShelfBehaviorHidden = "hidden"
	// ShelfBehaviorInvalid represents an invalid state.
	ShelfBehaviorInvalid = "invalid"
)

// SetShelfBehavior sets the shelf visibility behavior.
// displayID is the display that contains the shelf.
func SetShelfBehavior(ctx context.Context, c *chrome.Conn, displayID string, b ShelfBehavior) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setShelfAutoHideBehavior(%q, %q, function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  });
		})`, displayID, b)
	return c.EvalPromise(ctx, expr, nil)
}

// GetShelfBehavior returns the shelf visibility behavior.
// displayID is the display that contains the shelf.
func GetShelfBehavior(ctx context.Context, c *chrome.Conn, displayID string) (ShelfBehavior, error) {
	var b ShelfBehavior
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getShelfAutoHideBehavior(%q, function(behavior) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(behavior);
		    }
		  });
		})`, displayID)
	if err := c.EvalPromise(ctx, expr, &b); err != nil {
		return ShelfBehaviorInvalid, err
	}
	switch b {
	case ShelfBehaviorAlwaysAutoHide, ShelfBehaviorNeverAutoHide, ShelfBehaviorHidden:
	default:
		return ShelfBehaviorInvalid, errors.Errorf("invalid shelf behavior %q", b)
	}
	return b, nil
}

// ShelfAlignment represents the different Chrome OS shelf alignments.
type ShelfAlignment string

// As defined in ShelfAlignment here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/shelf_types.h
const (
	ShelfAlignmentBottom       ShelfAlignment = "Bottom"
	ShelfAlignmentLeft                        = "Left"
	ShelfAlignmentRight                       = "Right"
	ShelfAlignmentBottomLocked                = "BottomLocked"
	ShelfAlignmentInvalid                     = "Invalid"
)

// SetShelfAlignment sets the shelf alignment.
// displayID is the display that contains the shelf.
func SetShelfAlignment(ctx context.Context, c *chrome.Conn, displayID string, a ShelfAlignment) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setShelfAlignment(%q, %q, function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  });
		})`, displayID, a)
	return c.EvalPromise(ctx, expr, nil)
}

// GetShelfAlignment returns the shelf alignment.
// displayID is the display that contains the shelf.
func GetShelfAlignment(ctx context.Context, c *chrome.Conn, displayID string) (ShelfAlignment, error) {
	var a ShelfAlignment
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getShelfAlignment(%q, function(alignment) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(alignment);
		    }
		  });
		})`, displayID)
	if err := c.EvalPromise(ctx, expr, &a); err != nil {
		return ShelfAlignmentInvalid, err
	}
	switch a {
	case ShelfAlignmentBottom, ShelfAlignmentLeft, ShelfAlignmentRight, ShelfAlignmentBottomLocked:
	default:
		return ShelfAlignmentInvalid, errors.Errorf("invalid shelf alignment %q", a)
	}
	return a, nil
}
