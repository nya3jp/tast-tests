// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servo is used to communicate with servo devices connected to DUTs.
// It communicates with servod over XML-RPC.
// More details on servo: https://www.chromium.org/chromium-os/servo
package servo

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/divan/gorilla-xmlrpc/xml"
)

// Args are the method parameters for the XML-RPC call.
// TODO(jeffcarp): Support more method signatures.
type Args struct {
	// Message is the first string argument to the method.
	Message string
}

// Reply is the format of the expected response from servod.
type Reply struct {
	// Message contains the response value from servod.
	Message string
}

// ServodDefaultHost is the default host for servod. Currently unconfigurable.
const ServodDefaultHost = "localhost"

// ServodDefaultPort is the default port for servod. Currently unconfigurable.
const ServodDefaultPort = 9999

// Call makes an XML-RPC call to servod.
func Call(method string, args Args) (reply Reply, err error) {
	buf, _ := xml.EncodeClientRequest(method, &args)

	// TODO(jeffcarp): Parameterize servod host and port.
	servodURL := fmt.Sprintf("http://%s:%d", ServodDefaultHost,
		ServodDefaultPort)

	resp, err := http.Post(servodURL, "text/xml", bytes.NewBuffer(buf))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	err = xml.DecodeClientResponse(resp.Body, &reply)
	if err != nil {
		return
	}

	return
}
