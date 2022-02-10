// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package https

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
)

/*
The ServerConfiguration structure that stores all information to start the https server.
*/
type ServerConfiguration struct {
	Headers               map[string]string
	ServerKeyPath         string
	ServerCertificatePath string
	HostedFilesBasePath   string
	CaCertificatePath     string
}

func copyFile(source, destination string) error {
	bytesRead, err := ioutil.ReadFile(source)
	ioutil.WriteFile(destination, bytesRead, 0644)
	return err
}

type httpsHandler struct {
}

func (handler httpsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for key, value := range server.ServerConfiguration.Headers {
		w.Header().Set(key, value)
	}
	path := r.URL.String()
	b, err := ioutil.ReadFile(server.ServerConfiguration.HostedFilesBasePath + "/" + path[1:len(path)])
	if err != nil {
		w.WriteHeader(404)
	} else {
		w.WriteHeader(200)
		w.Write([]byte(b))
	}
}

/*
The Server structure contains all meta data and control objects for a https server.
*/
type Server struct {
	ServerConfiguration ServerConfiguration
	server              *http.Server
	Address             string
	Error               error
	handler             httpsHandler
}

var server Server

/*
Close shuts down a server that was started with the StartServer function.
*/
func (server Server) Close() error {
	return server.server.Close()
}

/*
StartServer starts up an https server without blocking.
Returns a Server instance containing
- the server configuration,
- the base address,
- an error object in case the server could not be started.
*/
func StartServer(configuration ServerConfiguration) Server {

	server = Server{
		ServerConfiguration: configuration,
		server:              &http.Server{},
		Address:             "",
		Error:               nil,
		handler:             httpsHandler{},
	}
	address, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		server.Error = err
		return server
	}
	listener, err := net.ListenTCP("tcp", address)
	if err != nil {
		server.Error = err
		return server
	}
	server.server.Handler = &server.handler
	go server.server.ServeTLS(listener, configuration.ServerCertificatePath, configuration.ServerKeyPath)
	server.Address = fmt.Sprintf("https://localhost:%d", listener.Addr().(*net.TCPAddr).Port)
	return server

}

/*
ConfigureChromeToAcceptCertificate adds the specified CA certificate to the authorities configuration of chrome.
The server certificate will then be accepted by chrome without complaining about security.
*/
func ConfigureChromeToAcceptCertificate(ctx context.Context, configuration ServerConfiguration, chrome *chrome.Chrome, targetBrowser *browser.Browser, testConnection *chrome.TestConn) error {

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
