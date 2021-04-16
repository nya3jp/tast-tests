// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package request

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Send sends a request to Omaha and parses the response.
func Send(ctx context.Context, req *Request, filePrefix string) (*Response, error) {
	reqData, err := xml.MarshalIndent(&req, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request")
	}

	outDir, outDirOk := testing.ContextOutDir(ctx)
	if outDirOk {
		if err := ioutil.WriteFile(filepath.Join(outDir, filePrefix+"-request.xml"), reqData, 0644); err != nil {
			testing.ContextLog(ctx, "Failed to dump request to file: ", err)
		}
	}

	res, err := http.Post(OmahaRequestURL, "application/xml", bytes.NewReader(reqData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}

	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed read response body")
	}

	if outDirOk {
		if err := ioutil.WriteFile(filepath.Join(outDir, filePrefix+"-response.xml"), resData, 0644); err != nil {
			return nil, errors.Wrap(err, "failed to dump request to file")
		}
	}

	var parsed Response
	if err := xml.Unmarshal(resData, &parsed); err != nil {
		return nil, errors.Wrap(err, "failed parse the response")
	}

	if outDirOk {
		parsedRes, err := json.MarshalIndent(parsed, "", "  ")
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal response")
		}

		if err := ioutil.WriteFile(filepath.Join(outDir, filePrefix+"-response-parsed.json"), parsedRes, 0644); err != nil {
			testing.ContextLog(ctx, "Failed to dump request to file: ", err)
		}
	}

	return &parsed, nil
}
