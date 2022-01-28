// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package safesearch

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

// ForceYouTubeRestrict policy values.
const (
	ForceYouTubeRestrictDisabled = iota
	ForceYouTubeRestrictModerate
	ForceYouTubeRestrictStrict
)

// There are 3 kinds of contents:
// - Strong content is restricted even for a moderate restriction.
// - Mild content is only restricted when strict restriction is set.
// - Friendly content is never restricted.
const (
	mildContent   = "https://www.youtube.com/watch?v=Fmwfmee2ZTE"
	strongContent = "https://www.youtube.com/watch?v=yR79oLrI1g4"
)

// TestYouTubeRestrictedMode checks whether strong and mild content is
// restricted as expected, and returns an error if it isn't.
func TestYouTubeRestrictedMode(ctx context.Context, br ash.ConnSource, expectedStrongContentRestricted, expectedMildContentRestricted bool) error {
	if mildContentRestricted, err := isYouTubeContentRestricted(ctx, br, mildContent); err != nil {
		return err
	} else if mildContentRestricted != expectedMildContentRestricted {
		return errors.Errorf("unexpected mild content restriction; got %t, wanted %t", mildContentRestricted, expectedMildContentRestricted)
	}

	if strongContentRestricted, err := isYouTubeContentRestricted(ctx, br, strongContent); err != nil {
		return err
	} else if strongContentRestricted != expectedStrongContentRestricted {
		return errors.Errorf("unexpected strong content restriction; got %t, wanted %t", strongContentRestricted, expectedStrongContentRestricted)
	}

	return nil
}

func isYouTubeContentRestricted(ctx context.Context, br ash.ConnSource, url string) (bool, error) {
	message, err := getYouTubeErrorMessage(ctx, br, url)
	if err != nil {
		return false, err
	}

	return message != "", nil
}

// getYouTubeErrorMessage returns the error message, if any, returned by Youtube while trying to view the given url.
func getYouTubeErrorMessage(ctx context.Context, br ash.ConnSource, url string) (string, error) {
	conn, err := br.NewConn(ctx, url)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	var message string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, `document.getElementById('error-screen').innerText`, &message); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  15 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return "", err
	}

	return strings.TrimSpace(message), nil
}
