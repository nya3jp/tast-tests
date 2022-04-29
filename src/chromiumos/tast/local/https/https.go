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
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
)

const (
	caCommonName              = "TastCA"
	caCertificateOrganization = "org-Google"
)

// The ServerConfiguration structure that stores all information to start the https server.
type ServerConfiguration struct {
	Headers               map[string]string
	ServerKeyPath         string
	ServerCertificatePath string
	HostedFilesBasePath   string
	CaCertificatePath     string
	PatternHandlers       []patternHandler // See HandleFunc().
}

func copyFile(source, destination string) error {
	bytesRead, err := ioutil.ReadFile(source)
	ioutil.WriteFile(destination, bytesRead, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to copy file")
	}
	return nil
}

type handlerFunction func(w http.ResponseWriter, r *http.Request)

type patternHandler struct {
	pattern string
	handler handlerFunction
}

type httpsHandler struct {
	serverID        int
	patternHandlers []patternHandler
}

// HandleFunc sets a custom HTTP handler function for a specific file.
// HandleFunc needs to be called before StartServer(). When the relative path of a requested file exactly matches
// |pattern|, e.g. "/index.html", the server will use the ResponseWriter to set all required HTTPS
// headers and then call the handlerFunction instead of directly serving the requested file.
func (config *ServerConfiguration) HandleFunc(pattern string, fun handlerFunction) {
	config.PatternHandlers = append(config.PatternHandlers, patternHandler{pattern, fun})
}

func (handler httpsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, server := range servers {
		if server.handler.serverID != handler.serverID {
			continue
		}
		for key, value := range server.ServerConfiguration.Headers {
			w.Header().Set(key, value)
		}
		path := r.URL.String()
		for _, ph := range handler.patternHandlers {
			if ph.pattern == path {
				ph.handler(w, r)
				return
			}
		}
		b, err := ioutil.ReadFile(server.ServerConfiguration.HostedFilesBasePath + "/" + path[1:len(path)])
		if err != nil {
			w.WriteHeader(404)
		} else {
			w.WriteHeader(200)
			w.Write([]byte(b))
		}
	}
}

// The Server structure contains all meta data and control objects for a https server.
type Server struct {
	ServerConfiguration ServerConfiguration
	server              *http.Server
	Address             string
	Error               error
	handler             httpsHandler
}

var servers []Server

// Close shuts down a server that was started with the StartServer function.
func (server Server) Close() error {
	return server.server.Close()
}

// StartServer starts up an https server without blocking.
// Returns a Server instance containing
// - the server configuration,
// - the base address,
// - an error object in case the server could not be started.
func StartServer(config ServerConfiguration) Server {
	server := Server{
		ServerConfiguration: config,
		server:              &http.Server{},
		Address:             "",
		Error:               nil,
		handler:             httpsHandler{patternHandlers: config.PatternHandlers, serverID: len(servers)},
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
	go server.server.ServeTLS(listener, config.ServerCertificatePath, config.ServerKeyPath)
	server.Address = fmt.Sprintf("https://localhost:%d", listener.Addr().(*net.TCPAddr).Port)
	servers = append(servers, server)
	return server
}

// ConfigureChromeToAcceptCertificate adds the specified CA certificate to the authorities configuration of chrome.
// The server certificate will then be accepted by chrome without complaining about security.
func ConfigureChromeToAcceptCertificate(ctx context.Context, config ServerConfiguration, cr *chrome.Chrome, br *browser.Browser, tconn *chrome.TestConn) error {
	// Don't add certificate if it already exists.
	if certExists, err := CertificateExists(ctx, cr, br, tconn, caCommonName, caCertificateOrganization); err != nil {
		return errors.Wrap(err, "failed to check if certificate exists")
	} else if certExists {
		return nil
	}

	_, caCertFileName := filepath.Split(config.CaCertificatePath)

	// Copy the certificate file to a local Downloads directory.
	certDest := filepath.Join(filesapp.MyFilesPath, filesapp.Downloads, caCertFileName)
	if err := copyFile(config.CaCertificatePath, certDest); err != nil {
		return errors.Wrap(err, "failed to copy certificate")
	}

	// Add the certificate in the certificate settings.
	policyutil.SettingsPage(ctx, cr, br, "certificates")
	ui := uiauto.New(tconn)
	authorities := nodewith.Name("Authorities").Role(role.Tab)
	authTabText := nodewith.Name("You have certificates on file that identify these certificate authorities").Role(role.StaticText)
	importButton := nodewith.Name("Import").Role(role.Button)
	certFileItem := nodewith.Name(caCertFileName).First()
	openButton := nodewith.Name("Open").Role(role.Button)
	trust1Checkbox := nodewith.NameContaining("Trust this certificate for identifying websites").Role(role.CheckBox)
	trust2Checkbox := nodewith.NameContaining("Trust this certificate for identifying email users").Role(role.CheckBox)
	trust3Checkbox := nodewith.NameContaining("Trust this certificate for identifying software makers").Role(role.CheckBox)
	okButton := nodewith.Name("OK").Role(role.Button).ClassName("action-button")

	if err := uiauto.Combine("set_cerficate",
		ui.WaitUntilExists(authorities),
		ui.LeftClick(authorities),
		ui.WaitUntilExists(authTabText),
		ui.WaitUntilExists(importButton),
		ui.LeftClick(importButton),
		ui.WaitUntilExists(certFileItem),
		ui.LeftClick(certFileItem),
		ui.WaitUntilExists(openButton),
		ui.LeftClick(openButton),
		ui.WaitUntilExists(trust1Checkbox),
		ui.WaitUntilExists(okButton),
		ui.LeftClick(trust1Checkbox),
		ui.LeftClick(trust2Checkbox),
		ui.LeftClick(trust3Checkbox),
		ui.LeftClick(okButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to set certificate")
	}

	// Check if addition was successful.
	if certExists, err := CertificateExists(ctx, cr, br, tconn, caCommonName, caCertificateOrganization); err != nil {
		return errors.Wrap(err, "failed to add certificate")
	} else if !certExists {
		return errors.Errorf("failed to add certificate with name %q", caCommonName)
	}

	return nil
}

// CertificateExists returns whether the certificate already exists.
func CertificateExists(ctx context.Context, cr *chrome.Chrome, br *browser.Browser, tconn *chrome.TestConn, certName, authOrgName string) (bool, error) {
	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to use keyboard")
	}
	defer kb.Close()

	policyutil.SettingsPage(ctx, cr, br, "certificates")
	authorities := nodewith.Name("Authorities").Role(role.Tab)
	authTabText := nodewith.Name("You have certificates on file that identify these certificate authorities").Role(role.StaticText)

	if err := uiauto.Combine("open correct tab",
		ui.WaitUntilExists(authorities),
		ui.LeftClick(authorities),
		ui.WaitUntilExists(authTabText),
	)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to open authorities tab")
	}

	certPanel := nodewith.Name(authOrgName).Role(role.InlineTextBox)

	// Authority of the cert is not found, so cert also cannot exist.
	if isFound, err := ui.IsNodeFound(ctx, certPanel); err != nil {
		return false, errors.Wrap(err, "failed to check if certificate authority node is found")
	} else if !isFound {
		return false, nil
	}

	// If collapsible panel entry exists, it means there is at least one cert under it,
	// which means that "More actions" button must appear when the panel has been opened.
	if err := uiauto.Combine("open collapsible panel to check if the cert is there",
		ui.MakeVisible(certPanel),
		ui.LeftClick(certPanel),
		kb.AccelAction("Tab"),
		kb.AccelAction("Enter"),
		ui.WaitUntilExists(nodewith.Role(role.Button).Name("More actions")),
	)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to open collapsible panel")
	}

	certText := nodewith.Name(certName).Role(role.StaticText)
	// The actual cert is not found.
	if isFound, err := ui.IsNodeFound(ctx, certText); err != nil {
		return false, errors.Wrap(err, "failed to check if certificate node is found")
	} else if !isFound {
		return false, nil
	}

	return true, nil
}
