// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// GoogleMeetConfig defines input params and retry settings for Google meet testing.
type GoogleMeetConfig struct {
	Account       string
	Password      string
	URLs          []string
	RetryTimeout  time.Duration
	RetryInterval time.Duration
}

// GetGoogleMeetConfig returns an object that contains the Google meet configuration.
func GetGoogleMeetConfig(ctx context.Context, s *testing.ServiceState, roomSize int) (GoogleMeetConfig, error) {
	// If roomSize is NoRoom, an empty object is returned.
	if roomSize == NoRoom {
		return GoogleMeetConfig{}, nil
	}
	const (
		defaultMeetRetryTimeout  = 40 * time.Minute
		defaultMeetRetryInterval = 2 * time.Minute
	)
	meetAccount, ok := s.Var("ui.meet_account")
	if !ok {
		return GoogleMeetConfig{}, errors.New("failed to get variable ui.meet_account")
	}
	// Convert to lower case because user account is case-insensitive and shown as lower case in CrOS.
	meetAccount = strings.ToLower(meetAccount)
	meetPassword, ok := s.Var("ui.meet_password")
	if !ok {
		return GoogleMeetConfig{}, errors.New("failed to get variable ui.meet_password")
	}

	var urlVar, urlSeondaryVar string
	switch roomSize {
	case SmallRoomSize:
		urlVar = "ui.meet_url_small"
		urlSeondaryVar = "ui.meet_url_small_secondary"
	case LargeRoomSize:
		urlVar = "ui.meet_url_large"
		urlSeondaryVar = "ui.meet_url_large_secondary"
	case ClassRoomSize:
		urlVar = "ui.meet_url_class"
		urlSeondaryVar = "ui.meet_url_class_secondary"
	default:
		urlVar = "ui.meet_url_two"
		urlSeondaryVar = "ui.meet_url_two_secondary"
	}
	varToURLs := func(varName, generalVarName string) []string {
		var urls []string
		varStr, ok := s.Var(varName)
		if !ok {
			// If specific meeting url is not found, try the general meet url var.
			if varStr, ok = s.Var(generalVarName); !ok {
				testing.ContextLogf(ctx, "Variable %q or %q is not provided", varName, generalVarName)
				return urls
			}
		}
		// Split to URLs and ignore empty ones.
		for _, url := range strings.Split(varStr, ",") {
			s := strings.TrimSpace(url)
			if s != "" {
				urls = append(urls, s)
			}
		}
		return urls
	}
	meetURLs := varToURLs(urlVar, "ui.meet_url")
	if len(meetURLs) == 0 {
		// Primary meet URL is mandatory.
		return GoogleMeetConfig{}, errors.New("no valid primary meet URLs are given")
	}
	meetSecURLs := varToURLs(urlSeondaryVar, "ui.meet_url_secondary")
	// Shuffle the URLs so different tests can try different URLs with random order.
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(meetURLs), func(i, j int) { meetURLs[i], meetURLs[j] = meetURLs[j], meetURLs[i] })
	rand.Shuffle(len(meetSecURLs), func(i, j int) { meetSecURLs[i], meetSecURLs[j] = meetSecURLs[j], meetSecURLs[i] })
	// Put secondary URLs to the tail.
	meetURLs = append(meetURLs, meetSecURLs...)
	testing.ContextLog(ctx, "Google meet URLs: ", meetURLs)

	varToDuration := func(name string, defaultValue time.Duration) (time.Duration, error) {
		str, ok := s.Var(name)
		if !ok {
			return defaultValue, nil
		}

		val, err := strconv.Atoi(str)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to parse integer variable %v", name)
		}

		return time.Duration(val) * time.Minute, nil
	}
	meetRetryTimeout, err := varToDuration("ui.meet_url_retry_timeout", defaultMeetRetryTimeout)
	if err != nil {
		return GoogleMeetConfig{}, errors.Wrapf(err, "failed to parse %q to time duration", defaultMeetRetryTimeout)
	}
	meetRetryInterval, err := varToDuration("ui.meet_url_retry_interval", defaultMeetRetryInterval)
	if err != nil {
		return GoogleMeetConfig{}, errors.Wrapf(err, "failed to parse %q to time duration", defaultMeetRetryInterval)
	}
	testing.ContextLogf(ctx, "Retry vars: meetRetryTimeout %v, meetRetryInterval %v", meetRetryTimeout, meetRetryInterval)

	return GoogleMeetConfig{
		Account:       meetAccount,
		Password:      meetPassword,
		URLs:          meetURLs,
		RetryTimeout:  meetRetryTimeout,
		RetryInterval: meetRetryInterval,
	}, nil
}
