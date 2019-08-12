// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package updateserver provides a fake update server implementation that can be used by tests.
package updateserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"text/template"

	"chromiumos/tast/lsbrelease"
)

// A sample response is at https://chromium.googlesource.com/chromiumos/platform/update_engine/+/refs/heads/master/sample_omaha_v3_response.xml
const responseTmpl = `<?xml version='1.0' encoding='UTF-8'?>
	<response protocol="3.0" server="nebraska">
		<daystart elapsed_days="4434" elapsed_seconds="53793" />
		<app appid="{{.AppID}}" status=""></app>
		<app appid="{{.AppID}}_{{.DLCModuleID}}" status="ok">
			<updatecheck status="ok">
			<urls>
				<url codebase="file:///usr/local/dlc/{{.DLCModuleID}}/test-package/" />
			</urls>
			<manifest version="{{.RelVersion}}">
				<actions>
					<action event="update" run="dlcservice_test-dlc.payload" />
					<action ChromeOSVersion="{{.RelVersion}}" ChromeVersion="1.0.0.0" IsDeltaPayload="false" event="postinstall" deadline="now" />
				</actions>
				<packages>
					<package fp="1.{{.PayloadHash}}" hash_sha256="{{.PayloadHash}}" name="dlcservice_test-dlc.payload" required="true" size="{{.PayloadSize}}" />
				</packages>
			</manifest>
			</updatecheck>
		</app>
	</response>`

func getPayloadData(dlcModuleID string) (payloadHash string, payloadSize string, err error) {
	payloadJSONFileBytes, err := ioutil.ReadFile("/usr/local/dlc/" + dlcModuleID + "/test-package/dlcservice_test-dlc.payload.json")
	if err != nil {
		return "", "", err
	}

	var payloadJSON map[string]*json.RawMessage
	err = json.Unmarshal(payloadJSONFileBytes, &payloadJSON)
	if err != nil {
		return "", "", err
	}

	// Set the payloadHash.
	var sha256 string
	err = json.Unmarshal(*payloadJSON["sha256_hex"], &sha256)
	if err != nil {
		return "", "", err
	}
	sha256Decoded, err := base64.StdEncoding.DecodeString(sha256)
	if err != nil {
		return "", "", err
	}
	payloadHash = hex.EncodeToString(sha256Decoded)

	// Set the payloadSize.
	var size int
	err = json.Unmarshal(*payloadJSON["size"], &size)
	if err != nil {
		return "", "", err
	}
	payloadSize = strconv.Itoa(size)

	return payloadHash, payloadSize, nil
}

// New returns a new httptest.Server that acts like an update server.
// The server is already started, but the caller must call its Close
// method to shut it down.
// |dlcModuleID| is used to construct appID of the DLC module and should only
// be test{1..2}-dlc as can be referenced in test-dlc ebuild.
func New(ctx context.Context, dlcModuleID string) (*httptest.Server, error) {
	// Loads payload data: hash, size.
	payloadHash, payloadSize, err := getPayloadData(dlcModuleID)
	if err != nil {
		return nil, err
	}

	// Loads response parameters.
	lsb, err := lsbrelease.Load()
	if err != nil {
		return nil, err
	}
	tmplData := struct {
		AppID       string
		RelVersion  string
		DLCModuleID string
		PayloadHash string
		PayloadSize string
	}{
		lsb[lsbrelease.ReleaseAppID],
		lsb[lsbrelease.Version],
		dlcModuleID,
		payloadHash,
		payloadSize,
	}

	// Constructs response.
	t := template.Must(template.New("resp").Parse(responseTmpl))
	var resp bytes.Buffer
	if err := t.Execute(&resp, tmplData); err != nil {
		return nil, err
	}

	// Starts the server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			fmt.Fprint(w, resp.String())
		default:
			http.Error(w, "Only POST requests are supported", http.StatusMethodNotAllowed)
		}
	}))
	return server, nil
}
