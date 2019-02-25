// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateserver

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"chromiumos/tast/testing"
)

// parseLsbRelease parses appid and target_version from lsb-release.
func parseLsbRelease(path string) (string, string, error) {
	lsbReleaseContent, err := ioutil.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(string(lsbReleaseContent), "\n")
	appid := ""
	targetVersion := ""
	for _, val := range lines {
		if slices := strings.SplitN(val, "=", 2); len(slices) == 2 {
			if slices[0] == "CHROMEOS_BOARD_APPID" {
				appid = slices[1]
			} else if slices[0] == "CHROMEOS_RELEASE_VERSION" {
				targetVersion = slices[1]
			}
		}
	}
	return appid, targetVersion, nil
}

// RunParseLsbRelease is a wrapper of parseLsbRelease.
func RunParseLsbRelease(path string) (string, string, error) {
	return parseLsbRelease(path)
}

// NewServer ensures update server is up and ready to accept update request.
func NewServer(ctx context.Context, dlcModuleID string) (*httptest.Server, error) {
	appid, targetVersion, err := parseLsbRelease("/etc/lsb-release")
	if err != nil {
		return nil, err
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			// A sample response is at https://chromium.googlesource.com/chromiumos/platform/update_engine/+/refs/heads/master/sample_omaha_v3_response.xml
			fmt.Fprintf(w, `<?xml version='1.0' encoding='UTF-8'?>
				<response protocol="3.0" server="nebraska">
				  <daystart elapsed_days="4434" elapsed_seconds="53793" />
				  <app appid="%s" status=""></app>
				  <app appid="%s_%s" status="ok">
				    <updatecheck status="ok">
				      <urls>
				        <url codebase="file:///usr/local/dlc/" />
				      </urls>
				      <manifest version="%s">
				        <actions>
				          <action event="update" run="dlcservice_test-dlc.payload" />
				          <action ChromeOSVersion="%s" ChromeVersion="1.0.0.0" IsDeltaPayload="false" MaxDaysToScatter="14" MetadataSignatureRsa="" MetadataSize="1" event="postinstall" />
				        </actions>
				        <packages>
				          <package fp="1.9f4290e6204eb12042b582a94a968bd565b11ae91f6bec717f0118c532293f62" hash_sha256="9f4290e6204eb12042b582a94a968bd565b11ae91f6bec717f0118c532293f62" name="dlcservice_test-dlc.payload" required="true" size="639" />
				        </packages>
				      </manifest>
				    </updatecheck>
				  </app>
				</response>`, appid, appid, dlcModuleID, targetVersion, targetVersion)
		default:
			fmt.Fprintf(w, "Only POST requests are supported")
		}
	}))

	// Waits for the update server to be ready.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if req, err := http.NewRequest("POST", server.URL, strings.NewReader("<request></request>")); err != nil {
			return err
		}
		client := &http.Client{}
		if resp, err := client.Do(req); err != nil {
			return err
		}
		defer resp.Body.Close()
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return server, err
	}
	return server, nil
}
