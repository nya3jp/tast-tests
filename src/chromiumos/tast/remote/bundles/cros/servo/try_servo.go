// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"bytes"
	"context"
	"log"
	"net/http"

	// "chromiumos/tast/dut"
	"chromiumos/tast/testing"

	// TODO(jeffcarp): Break out XML-RPC comms into a library
	"github.com/divan/gorilla-xmlrpc/xml"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TryServo,
		Desc: "Demonstrates running a test using Servo.",
		Contacts: []string{"jeffcarp@chromium.org", "derat@chromium.org", "tast-users@chromium.org"},
		Attr: []string{"informational"},
	})
}

func TryServo(ctx context.Context, s *testing.State) {
	method := "RutabagaService.hello"
	args := struct{
		Something string
	}{
		"asdf",
	}

	buf, _ := xml.EncodeClientRequest(method, &args)
	servoUrl := "http://localhost:1234/RPC2"

	resp, err := http.Post(servoUrl, "text/xml", bytes.NewBuffer(buf))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var reply string
	err = xml.DecodeClientResponse(resp.Body, &reply)
	if err != nil {
		return
	}

	log.Printf("Response: %v\n", reply)
}
