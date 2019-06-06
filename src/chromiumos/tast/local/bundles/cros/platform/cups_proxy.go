// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"net/http"
	"reflect"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CupsProxy,
		Desc: "Checks that cups_proxy proxy IPP requests as expected",
		Contacts: []string{
			"pihsun@chromium.org",     // Test author
			"tast-users@chromium.org", // Backup mailing list
		},
		Attr: []string{"informational"},
	})
}

func checkHeaderAndDelete(s *testing.State, headers map[string]string, key, value string) {
	actual, ok := headers[key]
	if !ok {
		s.Fatalf("expected header %q doesn't exist", key)
	}
	if actual != value {
		s.Fatalf("expected header %q = %q, found %q", key, value, actual)
	}
	delete(headers, key)
}

func CupsProxy(ctx context.Context, s *testing.State) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("failed to stop ui: ", err)
	}
	defer upstart.StartJob(ctx, "ui")

	if err := upstart.RestartJob(ctx, "cups_proxy"); err != nil {
		s.Fatal("failed to restart cups_proxy: ", err)
	}

	cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "test_echo_ipp_server")
	if err := cmd.Start(); err != nil {
		s.Fatal("failed to run test echo server: ", err)
	}
	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	ch, err := net.Dial("unix", "/run/cups_proxy/cups_proxy.sock")
	if err != nil {
		s.Fatal("failed to open unix socket: ", err)
	}
	defer ch.Close()

	body := "ABCDEFGHIJ"
	req, err := http.NewRequest("POST", "/", strings.NewReader(body))
	if err != nil {
		s.Fatal("failed to create IPP request: ", err)
	}

	headers := map[string]string{
		"Content-Type": "application/ipp",
		"Date":         "Mon, 04 Dec 2000 01:23:45 GMT",
		"User-Agent":   "CUPS/2.2.8 IPP/2.0",
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}
	req.Host = "localhost:0"

	if err := req.Write(ch); err != nil {
		s.Fatal("failed to write request: ", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(ch), req)
	if err != nil {
		s.Fatal("failed to read response: ", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		s.Fatal("response status code not 200")
	}

	respHeader := map[string]string{}
	for key, value := range resp.Header {
		if len(value) != 1 {
			s.Fatalf("found unexpected multi-value header: %q = %q", key, value)
		}
		respHeader[key] = value[0]
	}

	delete(respHeader, "Content-Length")
	checkHeaderAndDelete(s, respHeader, "Connection", "Keep-Alive")
	checkHeaderAndDelete(s, respHeader, "Host", req.Host)
	if !reflect.DeepEqual(respHeader, headers) {
		s.Fatalf("response header incorrect, expected %q, found %q", headers, respHeader)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respBody := buf.String()
	if respBody != body {
		s.Fatalf("response body incorrect, expected %q, found %q", body, respBody)
	}
}
