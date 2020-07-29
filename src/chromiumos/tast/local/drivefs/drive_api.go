// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"golang.org/x/oauth2/google"
	drive "google.golang.org/api/drive/v3"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// DriveAPI contains the stored client and Drive API service.
type DriveAPI struct {
	OauthClient *http.Client
	Service     *drive.Service
}

// CreateDriveAPI is a factory method that authorizes the logged in user.
// The factory returns a DriveAPI type that has helper methods to perform Drive API tasks.
func CreateDriveAPI(ctx context.Context, cr *chrome.Chrome, oauthCredentials string) (*DriveAPI, error) {
	config, err := google.ConfigFromJSON([]byte(oauthCredentials), drive.DriveFileScope)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing supplied oauth credentials")
	}

	// Create a channel that we can push the auth code into from the local server instance.
	ch := make(chan string)
	randState := fmt.Sprintf("st%d", time.Now().UnixNano())

	// Sets up a local server instance to handle the oauth redirect flow.
	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/favicon.ico" {
			http.Error(rw, "", 404)
			return
		}

		// Make sure the supplied state matches the redirected state.
		if req.FormValue("state") != randState {
			testing.ContextLogf(ctx, "Returned state does not match: req = %#v", req)
			http.Error(rw, "", 500)
			return
		}

		// If we have a code, put back into channel and respond success.
		if code := req.FormValue("code"); code != "" {
			fmt.Fprintf(rw, "success.")
			rw.(http.Flusher).Flush()
			ch <- code
			return
		}
		testing.ContextLog(ctx, "Redirect to server did not supply an auth code")
		http.Error(rw, "", 500)
	}))
	defer ts.Close()

	config.RedirectURL = ts.URL
	authURL := config.AuthCodeURL(randState)

	conn, err := cr.NewConn(ctx, authURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to navigate to auth url: %s", authURL)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// Wait for the profile element on oauth consent screen and click current user.
	if err := waitAndClickElem(ctx, conn, "document.querySelector('div[id=\"profileIdentifier\"]')"); err != nil {
		return nil, err
	}

	// Wait for the oauth scope dialog to show then click Allow.
	if err := waitAndClickElem(ctx, conn, "document.querySelector('div[data-custom-id=\"oauthScopeDialog-allow\"]')"); err != nil {
		return nil, err
	}

	// Wait for the oauth approval screen to show then click the final Allow.
	if err := waitAndClickElem(ctx, conn, "document.querySelector('div[id=\"submit_approve_access\"]')"); err != nil {
		return nil, err
	}

	code := <-ch

	// Exchange the supplied oauth credentials and auth code for oauth token
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, errors.Wrap(err, "failed to exchange the auth code")
	}

	// Generate a *http.Client from the retrieved oauth token.
	client := config.Client(ctx, token)

	// Generate the drive service with the supplied oauth client.
	service, err := drive.New(client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialise the drive client")
	}

	return &DriveAPI{
		OauthClient: client,
		Service:     service,
	}, nil
}

// CreateBlankGoogleDoc creates a google doc with supplied filename in the directory path.
// All paths should start with root unless they are team drives, in which case the drive path.
func (d *DriveAPI) CreateBlankGoogleDoc(ctx context.Context, fileName string, dirPath []string) error {
	doc := &drive.File{
		MimeType: "application/vnd.google-apps.document",
		Name:     fileName,
		Parents:  dirPath,
	}
	_, err := d.Service.Files.Create(doc).Do()

	if err != nil {
		return err
	}

	return nil
}

// waitAndClickElem simply waits until the element exists on the page.
// Once it exists it clicks the element supplied, the supplied element
// must be a singleton, this does not handle multiple elements.
func waitAndClickElem(ctx context.Context, conn *chrome.Conn, jsExpr string) error {
	if err := conn.WaitForExprFailOnErrWithTimeout(ctx, fmt.Sprintf("%s != null", jsExpr), time.Minute); err != nil {
		return errors.Wrapf(err, "failed waiting for html element selector to be non-null: %s", jsExpr)
	}

	if err := conn.Eval(ctx, fmt.Sprintf("%s.click()", jsExpr), nil); err != nil {
		return errors.Wrapf(err, "failed to click the html element selector: %s", jsExpr)
	}

	return nil
}
