// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servo is used to communicate with servo devices connected to DUTs.
// It communicates with servod over XML-RPC.
// More details on servo: https://www.chromium.org/chromium-os/servo
package servo

import (
	"bytes"
	"net/http"
	"github.com/divan/gorilla-xmlrpc/xml"
)

// Reply is the format of the expected response from servod.
type Reply struct {
    // Message contains the response value from servod.
    Message string
}

// Make an XML-RPC call to servod.
func Call(method string) (reply Reply, err error) {
	/*
	args := struct{
		Name string
		Value bool
	}{
		"uservo_pwr_en",
		true,
	}
	*/
	args := struct{}{}
	buf, _ := xml.EncodeClientRequest(method, &args)

	// TODO(jeffcarp): Parameterize servod host & port.
	servoUrl := "http://localhost:9999"

	resp, err := http.Post(servoUrl, "text/xml", bytes.NewBuffer(buf))
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
