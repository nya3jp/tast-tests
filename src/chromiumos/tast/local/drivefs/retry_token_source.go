// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"time"

	"golang.org/x/oauth2"
)

const (
	// RetryTokenSourceDefaultAttempts is the default number of retry attempts.
	RetryTokenSourceDefaultAttempts = 3
)

type retryTokenSource struct {
	MaxAttempts int
	Ctx         context.Context
	Delay       time.Duration
	src         oauth2.TokenSource
}

type retryTokenSourceOption interface {
	Apply(*retryTokenSource)
}

type withContext struct {
	ctx context.Context
}

func (w withContext) Apply(rts *retryTokenSource) {
	rts.Ctx = w.ctx
}

// WithContext builds an option that specifies a context to check for
// cancellation between retry attempts.
func WithContext(ctx context.Context) retryTokenSourceOption {
	return withContext{ctx: ctx}
}

type withDelay time.Duration

func (w withDelay) Apply(rts *retryTokenSource) {
	rts.Delay = time.Duration(w)
}

// WithDelay builds an option that specifies a delay between retry attempts.
func WithDelay(delay time.Duration) retryTokenSourceOption {
	return withDelay(delay)
}

type withMaxAttempts int

func (w withMaxAttempts) Apply(rts *retryTokenSource) {
	rts.MaxAttempts = int(w)
}

// WithMaxAttempts builds an option that overrides the default number of
// retry attempts.
func WithMaxAttempts(maxAttempts int) retryTokenSourceOption {
	if maxAttempts <= 0 {
		panic("Too few attempts")
	}
	return withMaxAttempts(maxAttempts)
}

// RetryTokenSource wraps the provided oauth2.TokenSource with one that retries
// failues to fetch tokens in the Token() function.
func RetryTokenSource(src oauth2.TokenSource, opts ...retryTokenSourceOption) oauth2.TokenSource {
	rts := &retryTokenSource{
		MaxAttempts: RetryTokenSourceDefaultAttempts,
		Ctx:         context.Background(), // NOLINT
		Delay:       0,
		src:         src,
	}
	for _, opt := range opts {
		opt.Apply(rts)
	}
	return rts
}

func (rts *retryTokenSource) Token() (tok *oauth2.Token, err error) {
	for attempts := 0; attempts < rts.MaxAttempts; attempts++ {
		tok, err = rts.src.Token()
		if err == nil {
			return tok, err
		}
		if rts.Ctx.Err() != nil {
			return tok, err
		}
		if rts.Delay > 0 {
			select {
			case <-time.After(rts.Delay):
			case <-rts.Ctx.Done():
				// Cancelled early, return the errors.
				return nil, rts.Ctx.Err()
			}
		}
	}
	return tok, err
}
