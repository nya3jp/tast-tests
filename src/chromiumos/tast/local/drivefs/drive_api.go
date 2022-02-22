// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"

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
