// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package isolatedapp

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
)

/*
The HTTPSServer structure that stores all information to start the https server.
*/
type HTTPSServer struct {
	Headers               map[string]string
	ServerKeyPath         string
	ServerCertificatePath string
	HostedFilesBasePath   string
	CaCertificatePath     string
}

var handler serverHandler

func copyFile(source, destination string) error {
	bytesRead, err := ioutil.ReadFile(source)
	ioutil.WriteFile(destination, bytesRead, 0644)
	return err
}

type serverHandler struct {
	ServerConfiguration HTTPSServer
}

func (handler serverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for key, value := range handler.ServerConfiguration.Headers {
		w.Header().Set(key, value)
	}
	path := r.URL.String()
	b, _ := ioutil.ReadFile(handler.ServerConfiguration.HostedFilesBasePath + "/" + path[1:len(path)])
	w.WriteHeader(200)
	w.Write([]byte(b))
}

/*
StartServer starts up an https server without blocking.
*/
func StartServer(configuration HTTPSServer) string {

	address, _ := net.ResolveTCPAddr("tcp", "localhost:0")
	listener, _ := net.ListenTCP("tcp", address)
	handler = serverHandler{configuration}
	go http.ServeTLS(listener, handler, configuration.ServerCertificatePath, configuration.ServerKeyPath)
	return "https://localhost:" + strconv.FormatInt(int64(listener.Addr().(*net.TCPAddr).Port), 10)

}

/*
ConfigureChromeToAcceptCertificate adds the specified CA certificate to the authorities configuration of chrome.
The server certificate will then be accepted by chrome without complaining about security.
*/
func ConfigureChromeToAcceptCertificate(ctx context.Context, configuration HTTPSServer, chrome *chrome.Chrome, targetBrowser *browser.Browser, testConnection *chrome.TestConn) error {

	certificateDestination := filesapp.MyFilesPath + "/" + filesapp.Downloads + "/ca-cert.pem"
	copyFile(configuration.CaCertificatePath, certificateDestination)
	policyutil.SettingsPage(ctx, chrome, targetBrowser, "certificates")
	ui := uiauto.New(testConnection)
	authorities := nodewith.NameContaining("Authorities").Role(role.Tab)
	importButton := nodewith.NameContaining("Import").Role(role.Button)
	certFileItem := nodewith.NameContaining("ca-cert.pem").First()
	openButton := nodewith.NameContaining("Open").Role(role.Button)
	trust1Checkbox := nodewith.NameContaining("Trust this certificate for identifying websites").Role(role.CheckBox)
	trust2Checkbox := nodewith.NameContaining("Trust this certificate for identifying email users").Role(role.CheckBox)
	trust3Checkbox := nodewith.NameContaining("Trust this certificate for identifying software makers").Role(role.CheckBox)
	okButton := nodewith.NameContaining("OK").Role(role.Button)
	if err := uiauto.Combine("set_cerficiate",
		ui.WaitUntilExists(authorities),
		ui.LeftClick(authorities),
		ui.WaitUntilExists(importButton),
		ui.LeftClick(importButton),
		ui.WaitUntilExists(certFileItem),
		ui.LeftClick(certFileItem),
		ui.WaitUntilExists(openButton),
		ui.LeftClick(openButton),
		ui.WaitUntilExists(trust1Checkbox),
		ui.WaitUntilExists(trust2Checkbox),
		ui.WaitUntilExists(trust3Checkbox),
		ui.WaitUntilExists(okButton),
		ui.LeftClick(trust1Checkbox),
		ui.LeftClick(trust2Checkbox),
		ui.LeftClick(trust3Checkbox),
		ui.LeftClick(okButton),
	)(ctx); err != nil {
		return err
	}
	return nil

}
