// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package request

import (
	"bytes"
	"context"
	"encoding/xml"
	"io/ioutil"
	"net/http"

	"github.com/google/uuid"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// New creates a new request with the ChromeOS constants filled in.
func New() *Request {
	return &Request{
		RequestID:      uuid.New(),
		SessionID:      uuid.New(),
		Protocol:       ProtocolVersion,
		Updater:        QAUpdaterID,
		UpdaterVersion: OmahaUpdaterVersion,
		IsMachine:      1,
		InstallSource:  InstallSourceScheduler,
		OS: OS{
			Version:  OSVersion,
			Platform: OSPlatform,
		},
	}
}

// Send sends a request to Omaha and parses the response.
func Send(ctx context.Context, req *Request) (*Response, error) {
	reqData, err := xml.MarshalIndent(&req, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request")
	}

	testing.ContextLog(ctx, "Omaha request: ", string(reqData))

	res, err := http.Post(OmahaRequestURL, "application/xml", bytes.NewReader(reqData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}

	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed read response body")
	}

	testing.ContextLog(ctx, "Omaha response: ", string(resData))

	var parsed Response
	if err := xml.Unmarshal(resData, &parsed); err != nil {
		return nil, errors.Wrap(err, "failed parse the response")
	}

	return &parsed, nil
}
