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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	drive "google.golang.org/api/drive/v3"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// APIClient contains the stored client and Drive API service.
type APIClient struct {
	token  *oauth2.Token
	config *oauth2.Config
}

// CreateAPIClient is a factory method that authorizes the logged in user.
// The factory returns a APIClient type that has helper methods to perform Drive API tasks.
func CreateAPIClient(ctx context.Context, cr *chrome.Chrome, oauthCredentials, refreshToken string) (*APIClient, error) {
	config, err := google.ConfigFromJSON([]byte(oauthCredentials), drive.DriveFileScope)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing supplied oauth credentials")
	}

	// Reconstruct the oauth token using just the refresh token.
	// Fortunately the refresh token never expires so it can continually be used
	// to get a new access token.
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	return &APIClient{
		token:  token,
		config: config,
	}, nil
}

// createNewDriveService is used to initialize a new drive client.
// It is setup before every API call to ensure the deadline context for a test
// is used in the *http.Client, otherwise it will take the fixture context and
// emit a context deadline exceeded on every call.
func (d *APIClient) createNewDriveService(ctx context.Context) (*drive.Service, error) {
	// Generate a *http.Client from the retrieved oauth token.
	client := d.config.Client(ctx, d.token)

	// Generate the drive service with the supplied oauth client.
	service, err := drive.New(client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialise the drive client")
	}

	return service, nil
}

// CreateBlankGoogleDoc creates a google doc with supplied filename in the directory path.
// All paths should start with root unless they are team drives, in which case the drive path.
func (d *APIClient) CreateBlankGoogleDoc(ctx context.Context, fileName string, dirPath []string) (*drive.File, error) {
	service, err := d.createNewDriveService(ctx)
	if err != nil {
		return nil, err
	}

	doc := &drive.File{
		MimeType: "application/vnd.google-apps.document",
		Name:     fileName,
		Parents:  dirPath,
	}
	return service.Files.Create(doc).Do()
}

// RemoveFileByID removes the file by supplied fileID.
func (d *APIClient) RemoveFileByID(ctx context.Context, fileID string) error {
	service, err := d.createNewDriveService(ctx)
	if err != nil {
		return err
	}

	return service.Files.Delete(fileID).Do()
}

// ListUserFiles lists all files in 'user' (files created by, opened by, or shared directly with the user).
func (d *APIClient) ListUserFiles(ctx context.Context) (*drive.FileList, error) {
	service, err := d.createNewDriveService(ctx)
	if err != nil {
		return nil, err
	}

	return service.Files.List().Corpora("user").Do()
}

// RenewRefreshTokenForAccount obtains a new OAuth refresh token for an account logged in
// on the chrome.Chrome instance. This is used by filemanager.DrivefsNewRefreshTokens
// test to easily obtain a set of new refresh tokens for the pooled GAIA logins.
func RenewRefreshTokenForAccount(ctx context.Context, cr *chrome.Chrome, oauthCredentials string) (string, error) {
	config, err := google.ConfigFromJSON([]byte(oauthCredentials), drive.DriveFileScope)
	if err != nil {
		return "", errors.Wrap(err, "failed parsing supplied oauth credentials")
	}

	// Create a channel that we can push the auth code into from the local server instance.
	authCodeChan := make(chan string)
	state := fmt.Sprintf("st%d", time.Now().UnixNano())

	// Sets up a local server instance to handle the oauth redirect flow.
	handler := serveAuthCodeRoute(ctx, state, authCodeChan)
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	// Generate the oauth consent URL.
	config.RedirectURL = ts.URL
	authURL := config.AuthCodeURL(state)

	// Start a renderer and navigate to the oauth consent URL.
	conn, err := cr.NewConn(ctx, authURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to navigate to auth url: %s", authURL)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// Wait for the profile element on oauth consent screen and click current user.
	if err := waitAndClickElement(ctx, conn, "document.querySelector('div[data-authuser=\"0\"]')"); err != nil {
		return "", err
	}

	// Wait for the oauth approval screen to show then click the final Allow.
	if err := waitAndClickElement(ctx, conn, "document.evaluate('//span[text()=\"Continue\"]', document, null, XPathResult.FIRST_ORDERED_NODE_TYPE).singleNodeValue"); err != nil {
		return "", err
	}

	authCode := <-authCodeChan

	// Exchange the supplied oauth credentials and auth code for oauth token.
	token, err := config.Exchange(ctx, authCode)
	if err != nil {
		return "", errors.Wrap(err, "failed to exchange the auth code")
	}

	return token.RefreshToken, nil
}

// serveAuthCodeRoute returns a http.Handler like function with state and auth code channel closed over.
func serveAuthCodeRoute(ctx context.Context, state string, authCodeChan chan string) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		if code := req.FormValue("code"); code != "" && req.FormValue("state") == state {
			rw.(http.Flusher).Flush()
			authCodeChan <- code
			return
		}

		http.Error(rw, "", 500)
	}
}

// waitAndClickElement simply waits until the element exists on the page.
// Once it exists it clicks the element supplied, the supplied element
// must be a singleton, this does not handle multiple elements.
func waitAndClickElement(ctx context.Context, conn *chrome.Conn, jsExpr string) error {
	if err := conn.WaitForExprFailOnErrWithTimeout(ctx, fmt.Sprintf("%s != null", jsExpr), time.Minute); err != nil {
		return errors.Wrapf(err, "failed waiting for html element selector to be non-null: %s", jsExpr)
	}

	if err := conn.Eval(ctx, fmt.Sprintf("%s.click()", jsExpr), nil); err != nil {
		return errors.Wrapf(err, "failed to click the html element selector: %s", jsExpr)
	}

	return nil
}
