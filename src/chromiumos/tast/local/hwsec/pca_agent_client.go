// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// PCAAgentClient delegates the request handling to the pca_agent_client command line tool.
type PCAAgentClient struct{}

// NewPCAAgentClient creates a new instance of RealVA.
func NewPCAAgentClient() *PCAAgentClient {
	return &PCAAgentClient{}
}

// HandleEnrollRequest calls pca_agent_client to process the enroll request.
func (rp *PCAAgentClient) HandleEnrollRequest(ctx context.Context, request string, pcaType hwsec.PCAType) (string, error) {
	// Set up input/output temp files.
	fdIn, err := ioutil.TempFile("", "tast-hwsec-test-pca-enroll-request")
	if err != nil {
		return "", errors.Wrap(err, "error creating temp file")
	}
	defer os.Remove(fdIn.Name())
	defer fdIn.Close()

	fdOut, err := ioutil.TempFile("", "tast-hwsec-test-pca-enroll-response")
	if err != nil {
		return "", errors.Wrap(err, "error creating temp file")
	}
	defer os.Remove(fdOut.Name())
	defer fdOut.Close()

	//Write the input file.
	if err := ioutil.WriteFile(fdIn.Name(), []byte(request), 0644); err != nil {
		return "", errors.Wrap(err, "failed to write input file")
	}

	// Execute the command.
	if _, err := testexec.CommandContext(ctx, "pca_agent_client", "enroll", "--input="+fdIn.Name(), "--output="+fdOut.Name()).Output(); err != nil {
		return "", errors.Wrap(err, "failed to call pca_agent_client")
	}

	// Read the output file content.
	output, err := ioutil.ReadFile(fdOut.Name())
	if err != nil {
		return "", errors.Wrap(err, "failed to read output")
	}
	return string(output), err
}

// HandleCertificateRequest calls pca_agent_client to process the certificate request.
func (rp *PCAAgentClient) HandleCertificateRequest(ctx context.Context, request string, pcaType hwsec.PCAType) (string, error) {
	// Set up input/output temp files.
	fdIn, err := ioutil.TempFile("", "tast-hwsec-test-pca-cert-request")
	if err != nil {
		return "", errors.Wrap(err, "error creating temp file")
	}
	defer os.Remove(fdIn.Name())
	defer fdIn.Close()

	fdOut, err := ioutil.TempFile("", "tast-hwsec-test-pca-cert-respone")
	if err != nil {
		return "", errors.Wrap(err, "error creating temp file")
	}
	defer os.Remove(fdOut.Name())
	defer fdOut.Close()

	//Write the input file.
	if err := ioutil.WriteFile(fdIn.Name(), []byte(request), 0644); err != nil {
		return "", errors.Wrap(err, "failed to write input file")
	}

	// Execute the command.
	if _, err := testexec.CommandContext(ctx, "pca_agent_client", "get_certificate", "--input="+fdIn.Name(), "--output="+fdOut.Name()).Output(); err != nil {
		return "", errors.Wrap(err, "failed to call pca_agent_client")
	}

	// Read the output file content.
	output, err := ioutil.ReadFile(fdOut.Name())
	if err != nil {
		return "", errors.Wrap(err, "failed to read output")
	}
	return string(output), err
}
