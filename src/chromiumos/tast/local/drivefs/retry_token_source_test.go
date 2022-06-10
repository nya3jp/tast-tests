// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"chromiumos/tast/errors"
)

type fakeTokenSource struct {
	err            error
	attempts       int
	attemptsToFail int
}

func newFakeTokenSource(err error, attemptsToFail int) *fakeTokenSource {
	return &fakeTokenSource{
		err:            err,
		attempts:       0,
		attemptsToFail: attemptsToFail,
	}
}

func (fts *fakeTokenSource) Token() (*oauth2.Token, error) {
	fts.attempts++
	if fts.attempts <= fts.attemptsToFail {
		return nil, fts.err
	}
	return &oauth2.Token{
		AccessToken:  "access_token",
		TokenType:    "Bearer",
		RefreshToken: "refresh_token",
	}, nil
}

func TestRetryTokenSourceRetriesFailures(t *testing.T) {
	fts := newFakeTokenSource(errors.New("simulated error"), 2)
	rts := RetryTokenSource(fts)
	tok, err := rts.Token()
	if err != nil {
		t.Errorf("expected error to be retried, but got: %s", err)
	}
	if tok == nil {
		t.Errorf("expected a token, but got nil")
	}
}

func TestRetryTokenSourceRetriesFailuresWithDelay(t *testing.T) {
	fts := newFakeTokenSource(errors.New("simulated error"), 2)
	rts := RetryTokenSource(fts, WithDelay(time.Nanosecond))
	tok, err := rts.Token()
	if err != nil {
		t.Errorf("expected error to be retried, but got: %s", err)
	}
	if tok == nil {
		t.Errorf("expected a token, but got nil")
	}
}

func TestRetryTokenSourceFailsAfterMaxAttempts(t *testing.T) {
	fts := newFakeTokenSource(errors.New("simulated error"), 2)
	rts := RetryTokenSource(fts, WithMaxAttempts(1))
	tok, err := rts.Token()
	if err == nil {
		t.Errorf("expected error")
	}
	if tok != nil {
		t.Errorf("expected nil token, but %v", tok)
	}
}

func TestRetryTokenSourceFailsWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fts := newFakeTokenSource(errors.New("simulated error"), 2)
	rts := RetryTokenSource(fts, WithContext(ctx))
	tok, err := rts.Token()
	if err == nil {
		t.Errorf("expected error")
	}
	if tok != nil {
		t.Errorf("expected nil token, but %v", tok)
	}

}
