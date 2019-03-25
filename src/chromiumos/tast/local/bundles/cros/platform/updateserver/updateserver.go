// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package updateserver provides a fake update server implementation that can be used by tests.
package updateserver

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
				<url codebase="file:///usr/local/dlc/" />
			</urls>
			<manifest version="{{.RelVersion}}">
				<actions>
					<action event="update" run="dlcservice_test-dlc.payload" />
					<action ChromeOSVersion="{{.RelVersion}}" ChromeVersion="1.0.0.0" IsDeltaPayload="false" event="postinstall" />
				</actions>
				<packages>
					<package fp="1.ceceb8c41d2493060f145046060de38735bd6f2a70b507ab3c3557c3fe62c142" hash_sha256="ceceb8c41d2493060f145046060de38735bd6f2a70b507ab3c3557c3fe62c142" name="dlcservice_test-dlc.payload" required="true" size="792" />
				</packages>
			</manifest>
			</updatecheck>
		</app>
	</response>`

// New returns a new httptest.Server that acts like an update server.
// The server is already started, but the caller must call its Close
// method to shut it down.
// |dlcModuleID| is used to construct appID of the DLC module.
func New(ctx context.Context, dlcModuleID string) (*httptest.Server, error) {
	// Loads response parameters.
	lsb, err := lsbrelease.Load()
	if err != nil {
		return nil, err
	}
	tmplData := struct {
		AppID       string
		RelVersion  string
		DLCModuleID string
	}{
		lsb[lsbrelease.ReleaseAppID],
		lsb[lsbrelease.Version],
		dlcModuleID,
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
