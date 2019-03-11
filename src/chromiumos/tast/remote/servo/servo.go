// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"bytes"
	"net/http"

	"io/ioutil"
	"chromiumos/tast/testing"

	"github.com/divan/gorilla-xmlrpc/xml"
)

func Call(method string, s *testing.State) (reply struct{Message string}, err error) {
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
	// s.Log("SENDING BUFFER:", string(buf))

	// TODO(jeffcarp): Parameterize servod host & port.
	servoUrl := "http://localhost:9999"

	resp, err := http.Post(servoUrl, "text/xml", bytes.NewBuffer(buf))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	/* DEBUGGING */
    bodyBytes, err := ioutil.ReadAll(resp.Body)
	s.Log("GOT BUFFER:", string(bodyBytes))
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	//*/

	err = xml.DecodeClientResponse(resp.Body, &reply)
	if err != nil {
		return
	}

	return
}
