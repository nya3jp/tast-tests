// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package versionutil provides utilities for the chrome and lacros versions
package versionutil

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ListReleasesResponse is imported from google.installer.versionhistory.v1.ListReleasesResponse
type ListReleasesResponse struct {
	Releases      []Release `json:"releases"`
	NextPageToken string    `json:"nextPageToken"`
}

// Release is imported from google.installer.versionhistory.v1.Release
type Release struct {
	Name    string `json:"name"`
	Serving struct {
		StartTime string `json:"startTime"`
	} `json:"serving"`
	Fraction      int    `json:"fraction"`
	Version       string `json:"version"`
	FractionGroup string `json:"fractionGroup"`
}

// HTTPGetVersionHistory calls VersionHistory API to get the latest live versions of all channels,
// returns a sorted list of `Release` by versions in ascending order.
// See https://developer.chrome.com/docs/versionhistory/reference/ for the usage of the API.
func HTTPGetVersionHistory(ctx context.Context) ([]Release, error) {
	const apiURL = "https://versionhistory.googleapis.com/v1/chrome/platforms/lacros/channels/all/versions/all/releases" +
		"?filter=endtime=none" +
		"&order_by=version%20asc" // in ascending order of version
	testing.ContextLog(ctx, "Calling VersionHistory: ", apiURL)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call VersionHistory API")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to call VersionHistory API, status: %s", resp.Status)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "error reading data")
	}

	releasesResp := ListReleasesResponse{}
	if err := json.Unmarshal(body, &releasesResp); err != nil {
		return nil, errors.Wrapf(err, "failed to parse VersionHistory API response: %v", string(body))
	}
	if len(releasesResp.Releases) == 0 {
		return nil, errors.New("failed to find any releases in VersionHistory API")
	}
	return releasesResp.Releases, nil
}

// CompatibleLacrosChannels returns a map of lacros channels to the latest live versions that are compatible with the given Ash version.
func CompatibleLacrosChannels(ctx context.Context, ashVersion string) (map[string]string, error) {
	// Call VersionHistory API to get the latest live versions of all channels.
	releases, err := HTTPGetVersionHistory(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call VersionHistory API")
	}

	// Parse the latest versions for each channel, and save the ones only compatible with Ash.
	channelToVersionMap := make(map[string]string)
	var channelRE = regexp.MustCompile(`chrome/platforms/lacros/channels/(\w+)`)
	for _, release := range releases {
		match := channelRE.FindStringSubmatch(release.Name)
		if match == nil {
			return nil, errors.New("failed to find channel in release")
		}
		channel := match[1]

		switch channel {
		case "canary", "dev", "beta", "stable":
			lacrosVersion := release.Version
			if ok, err := LacrosCompatibleWithAsh(lacrosVersion, ashVersion); err != nil {
				return nil, errors.Wrap(err, "failed to check version skew")
			} else if ok {
				// Keep the only latest version per channel.
				// If the multiple versions exist it is okay to overwrite the version here since HTTPGetVersionHistory returns a sorted list of `Release` by versions in ascending order.
				channelToVersionMap[channel] = lacrosVersion
			} else {
				testing.ContextLogf(ctx, "Skipping incompatible channel: %v, version: %v", channel, lacrosVersion)
			}
		default:
			testing.ContextLog(ctx, "Skipping unexpected channel: ", channel)
		}
	}
	return channelToVersionMap, nil
}

// LacrosCompatibleWithAsh compares Ash and Lacros versions and returns whether it is compatible with each other based on the version skew policy.
func LacrosCompatibleWithAsh(lacros, ash string) (bool, error) {
	// TODO(b/255344023): Utilize `version.go` to parse and compare versions.
	conv := func(version string) (parts [4]int, err error) {
		for id, part := range strings.Split(version, ".") {
			number, err := strconv.Atoi(part)
			if err != nil {
				return [4]int{0, 0, 0, 0}, err
			}
			parts[id] = number
		}
		return parts, nil
	}

	l, err := conv(lacros)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse lacros version: %v", lacros)
	}
	a, err := conv(ash)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse ash version: %v", ash)
	}

	// On the same milestone, compare the build number (NNNN) in x.x.NNNN.x.
	//	On the tip of tree, Ash and Lacros versions should be identical with .0 in the patch number.
	//	On release branches, Ash and Lacros support limited backwards as long as they are on the same branch with the same build number.
	// Examples:
	//	ash 100.0.1000.0, lacros 100.0.1000.0 => compatible because they are on the tip of tree with zero skew.
	//	ash 100.0.1000.9, lacros 100.0.1000.1 => compatible because they are on the same branch with the same build number (1000) even though Lacros patch number (.1) is older than Ash (.9)
	//	ash 100.0.1000.0, lacros 100.0.0999.0 => incompatible because they are not on the same branch with different build number.
	if l[0] == a[0] {
		return l[1] > a[1] || (l[1] == a[1] && l[2] >= a[2]), nil
	}

	// In the milestone skews, compare the milestone number (M) in M.x.xxxx.x and
	// confirm that Ash milestone X <= Lacros milestone Y <= Ash milestone X+2.
	// Examples:
	//	ash 100.x.xxxx.x, lacros 102.x.xxxx.x => compatible because lacros' milestone (102) > ash (100) within [0,2] of milestone skews
	//	ash 100.x.xxxx.x, lacros 103.x.xxxx.x => incompatible because lacros' milestone (103) > ash (100) not within [0,2] of milestone skews
	//	ash 100.x.xxxx.x, lacros  99.x.xxxx.x => incompatible because lacros' milestone (99) < ash (100)
	const maxMajorVersionSkew = 2
	return l[0] > a[0] && l[0] <= a[0]+maxMajorVersionSkew, nil
}
