// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateserver

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// EnsureUpdateServerUp ensures update server is up and ready to accept update request.
func EnsureUpdateServerUp(ctx context.Context, s *testing.State, dlcModuleID string, updateServerPort string) {
	// parseLsbReleases reads lsb-release info and parses the CHROMEOS_BOARD_APPID and CHROMEOS_RELEASE_VERSION info.
	parseLsbRelease := func() (string, string, error) {
		lsbReleaseFile, err := os.Open("/etc/lsb-release")
		if err != nil {
			return "", "", err
		}
		lsbReleaseContent, err := ioutil.ReadAll(lsbReleaseFile)
		if err != nil {
			return "", "", err
		}
		lsbReleaseContentSlice := strings.Split(string(lsbReleaseContent), "\n")
		appid := ""
		targetVersion := ""
		for _, val := range lsbReleaseContentSlice {
			if len(val) > 20 && val[:20] == "CHROMEOS_BOARD_APPID" {
				appid = val[21:]
			} else if len(val) > 24 && val[:24] == "CHROMEOS_RELEASE_VERSION" {
				targetVersion = val[25:]
			}
		}
		return appid, targetVersion, nil
	}
	// handleFunction handles update server HTTP request.
	handleFunction := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			appid, targetVersion, err := parseLsbRelease()
			if err != nil {
				s.Fatal("Failed to parse lsb-release: ", err)
				return
			}
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
			s.Fatal("Only POST requests are supported")
		}
	}

	// Starts the update server.
	http.HandleFunc("/", handleFunction)
	go func() {
		if err := http.ListenAndServe(":"+updateServerPort, nil); err != nil {
			s.Fatal("Failed to start http server: ", err)
		}
	}()

	// Waits for the update server to be ready.
	testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "curl", "-H", "\"Accept: application/xml\"", "-H", "\"Content-Type: application/xml\"", "-X", "POST", "-d", "<request></request>", "http://127.0.0.1:"+updateServerPort)
		if _, err := cmd.CombinedOutput(); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}
