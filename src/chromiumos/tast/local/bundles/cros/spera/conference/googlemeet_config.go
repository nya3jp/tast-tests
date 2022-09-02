// Copyright 2022 The ChromiumOS Authors.
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
	BondEnabled   bool
	BondCreds     []byte
	URLs          []string
	RetryTimeout  time.Duration
	RetryInterval time.Duration
}

// GetGoogleMeetConfig returns an object that contains the Google meet configuration.
func GetGoogleMeetConfig(ctx context.Context, s *testing.ServiceState, roomType RoomType) (GoogleMeetConfig, error) {
	// If roomType is NoRoom, an empty object is returned.
	if roomType == NoRoom {
		return GoogleMeetConfig{}, nil
	}
	const (
		defaultMeetRetryTimeout  = 40 * time.Minute
		defaultMeetRetryInterval = 2 * time.Minute
	)
	meetAccount, ok := s.Var("spera.meet_account")
	if !ok {
		return GoogleMeetConfig{}, errors.New("failed to get variable spera.meet_account")
	}
	// Convert to lower case because user account is case-insensitive and shown as lower case in CrOS.
	meetAccount = strings.ToLower(meetAccount)
	meetPassword, ok := s.Var("spera.meet_password")
	if !ok {
		return GoogleMeetConfig{}, errors.New("failed to get variable spera.meet_password")
	}

	var bondEnabled bool
	bondEnabledStr, ok := s.Var("spera.GoogleMeetCUJ.bond_enabled")
	if ok && bondEnabledStr == "true" {
		bondEnabled = true
	} else {
		bondEnabled = false
	}
	bondCreds, ok := s.Var("spera.GoogleMeetCUJ.bond_key")
	if !ok || len(bondCreds) < 1 {
		if bondEnabled {
			return GoogleMeetConfig{}, errors.New("bond API is enabled via spera.GoogleMeetCUJ.bond_enabled but spera.GoogleMeetCUJ.bond_key is not set")
		}
		bondCreds = ""
	}

	var urlVar, urlSeondaryVar string
	switch roomType {
	case TwoRoomSize:
		urlVar = "spera.meet_url_two"
		urlSeondaryVar = "spera.meet_url_two_secondary"
	case SmallRoomSize:
		urlVar = "spera.meet_url_small"
		urlSeondaryVar = "spera.meet_url_small_secondary"
	case LargeRoomSize:
		urlVar = "spera.meet_url_large"
		urlSeondaryVar = "spera.meet_url_large_secondary"
	case ClassRoomSize:
		urlVar = "spera.meet_url_class"
		urlSeondaryVar = "spera.meet_url_class_secondary"
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
	meetURLs := varToURLs(urlVar, "spera.meet_url")
	if len(meetURLs) == 0 && bondCreds == "" {
		// Primary meet URL is mandatory.
		return GoogleMeetConfig{}, errors.New("neither valid primary meet URLs nor BOND credentials are given")
	}
	meetSecURLs := varToURLs(urlSeondaryVar, "spera.meet_url_secondary")
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
	meetRetryTimeout, err := varToDuration("spera.meet_url_retry_timeout", defaultMeetRetryTimeout)
	if err != nil {
		return GoogleMeetConfig{}, errors.Wrapf(err, "failed to parse %q to time duration", defaultMeetRetryTimeout)
	}
	meetRetryInterval, err := varToDuration("spera.meet_url_retry_interval", defaultMeetRetryInterval)
	if err != nil {
		return GoogleMeetConfig{}, errors.Wrapf(err, "failed to parse %q to time duration", defaultMeetRetryInterval)
	}
	testing.ContextLogf(ctx, "Retry vars: meetRetryTimeout %v, meetRetryInterval %v", meetRetryTimeout, meetRetryInterval)

	return GoogleMeetConfig{
		Account:       meetAccount,
		Password:      meetPassword,
		BondEnabled:   bondEnabled,
		BondCreds:     []byte(bondCreds),
		URLs:          meetURLs,
		RetryTimeout:  meetRetryTimeout,
		RetryInterval: meetRetryInterval,
	}, nil
}
