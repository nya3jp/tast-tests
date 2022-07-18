// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fastpair contains local Tast tests that test Fast Pair features.
package fastpair

import (
    "context"
    "time"

    "chromiumos/tast/local/chrome"
    "chromiumos/tast/local/chrome/crossdevice"
    "chromiumos/tast/local/chrome/crossdevice/smartlock"
    "chromiumos/tast/local/chrome/uiauto/faillog"
    "chromiumos/tast/testing"
)

func init() {
    testing.AddTest(&testing.Test{
        Func:         GetSavedDevices,
        LacrosStatus: testing.LacrosVariantUnneeded,
        Desc:         "Tests ability to get Saved Devices from Settings",
        Contacts: []string{
            "dclasson@google.com",
            "chromeos-sw-engprod@google.com",
            "chromeos-cross-device-eng@google.com",
        },
        Attr:         []string{"group:cross-device", "informational"},
        SoftwareDeps: []string{"chrome"},
        Timeout:      8 * time.Minute,
    })
}

const {
    // Links to the Footprints API.
    footprintsURL = "https://nearbydevices-pa.googleapis.com/v1/user/devices/alt=proto"
    // footprintsURL = "https://nearbydevices-pa.googleapis.com/v1/user/devices/?key=%s&alt=proto"
}

// GetSavedDevices tests getting Saved Devices from Settings..
func GetSavedDevices(ctx context.Context, s *testing.State) {
    user := "crosautotestfastpair2@gmail.com"
    pass := "danielclasson123!"
    apiKey := "AIzaSyDJe1-z0swcCDdaTnyDHiSF_6_Nr-rDqdg"

    cr, err := chrome.New(ctx, 
        chrome.GAIALogin(chrome.Creds{User: user, Pass: pass})
        chrome.EnableFeatures("FastPair", "FastPairSoftwareScanning"))

    if err != nil {
        s.Fatal("Chrome login failed: ", err)
    }
    s.Log("Logged in!")

    tconn, err := cr.TestAPIConn(ctx)
    if err != nil {
        s.Fatal("Failed to create test API connection: ", err)
    }
    s.Log("API connection worked!")

    client := &http.Client{}
    resp, err := fetchSavedDevices(client, apiKey)
    if err != nil {
        s.Fatal("Failed to fetch saved devices: ", err)
    }
}

// fetchSavedDevices makes a request to |footprintsURL| with the provided access token.
func fetchSavedDevices(client *http.Client, apiKey string) ([]byte, error) {
    url := fmt.Sprintf("%s?key=%s&alt=proto", footprintsURL, apiKey)
    req, err := http.NewRequest("GET", fetchEpochURL, nil)
    if err != nil {
        return nil, errors.Wrap(err, "failed to create http request")
    }
    // req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
    req.Header.Add("Content-Type", "application/x-protobuf")

    resp, err := client.Do(req)
    if err != nil {
        return nil, errors.Wrap(err, "failed to send the request")
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, errors.Errorf("failed with status %v", resp.Status)
    }

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, errors.Wrap(err, "failed to read response body")
    }

    return body, nil
}