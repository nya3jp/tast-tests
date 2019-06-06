// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"

	"chromiumos/tast/errors"
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

type header map[string]string

func checkHeaderAndDelete(headers header, key, value string) error {
	actual, ok := headers[key]
	if !ok {
		return errors.Errorf("expected header %q doesn't exist", key)
	}
	if actual != value {
		return errors.Errorf("expected header %q = %q, found %q", key, value, actual)
	}
	delete(headers, key)
	return nil
}

func createRequest(headers header, host string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest("POST", "/", body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create IPP request")
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	req.Host = host
	return req, nil
}

func checkResponse(resp *http.Response, headers header, host, body string) error {
	if resp.StatusCode != 200 {
		return errors.New("response status code not 200")
	}

	respHeader := header{}
	for key, value := range resp.Header {
		if len(value) != 1 {
			return errors.Errorf("found unexpected multi-value header: %q = %q", key, value)
		}
		respHeader[key] = value[0]
	}

	delete(respHeader, "Content-Length")
	if err := checkHeaderAndDelete(respHeader, "Connection", "Keep-Alive"); err != nil {
		return err
	}
	if err := checkHeaderAndDelete(respHeader, "Host", host); err != nil {
		return err
	}
	if !reflect.DeepEqual(respHeader, headers) {
		return errors.Errorf("response header incorrect, expected %q, found %q", headers, respHeader)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respBody := buf.String()
	if respBody != body {
		return errors.Errorf("response body incorrect, expected %q, found %q", body, respBody)
	}

	return nil
}

func read100ContinueResponse(reader *bufio.Reader, req *http.Request) error {
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		return errors.Wrap(err, "failed to read response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 100 {
		return errors.New("response status code not 100")
	}

	return nil
}

func testEcho(ctx context.Context, s *testing.State, ch net.Conn) {
	body := "ABCDEFGHIJ"
	headers := header{
		"Content-Type": "application/ipp",
		"Date":         "Mon, 04 Dec 2000 01:23:45 GMT",
		"User-Agent":   "CUPS/2.2.8 IPP/2.0",
	}
	host := "localhost:0"

	req, err := createRequest(headers, host, strings.NewReader(body))
	if err != nil {
		s.Fatal("failed to create request: ", err)
	}

	if err := req.Write(ch); err != nil {
		s.Fatal("failed to write request: ", err)
	}

	reader := bufio.NewReader(ch)

	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		s.Fatal("failed to read response: ", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp, headers, host, body); err != nil {
		s.Fatal("failed to verify response: ", err)
	}
}

func test100Continue(ctx context.Context, s *testing.State, ch net.Conn) {
	body := "ABCDEFGHIJ"
	headers := header{
		"Content-Type": "application/ipp",
		"Date":         "Mon, 04 Dec 2000 01:23:45 GMT",
		"User-Agent":   "CUPS/2.2.8 IPP/2.0",
		"Expect":       "100-continue",
	}
	host := "localhost:0"

	req, err := createRequest(headers, host, strings.NewReader(body))
	if err != nil {
		s.Fatal("failed to create request: ", err)
	}

	if err := req.Write(ch); err != nil {
		s.Fatal("failed to write request: ", err)
	}

	reader := bufio.NewReader(ch)

	if err := read100ContinueResponse(reader, req); err != nil {
		s.Fatal("failed to find 100 continue response: ", err)
	}
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		s.Fatal("failed to read response: ", err)
	}
	defer resp.Body.Close()

	// There should be no Expect: in response header.
	delete(headers, "Expect")
	if err := checkResponse(resp, headers, host, body); err != nil {
		s.Fatal("failed to verify response: ", err)
	}
}

func testChunkedRequest(ctx context.Context, s *testing.State, ch net.Conn) {
	body := "ABCDEFGHIJ"
	headers := header{
		"Content-Type": "application/ipp",
		"Date":         "Mon, 04 Dec 2000 01:23:45 GMT",
		"User-Agent":   "CUPS/2.2.8 IPP/2.0",
	}
	host := "localhost:0"

	req, err := createRequest(headers, host, strings.NewReader(body))
	if err != nil {
		s.Fatal("failed to create request: ", err)
	}
	req.ContentLength = -1

	if err := req.Write(ch); err != nil {
		s.Fatal("failed to write request: ", err)
	}

	reader := bufio.NewReader(ch)

	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		s.Fatal("failed to read response: ", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp, headers, host, body); err != nil {
		s.Fatal("failed to verify response: ", err)
	}
}

func CupsProxy(ctx context.Context, s *testing.State) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.StartJob(ctx, "ui")

	if err := upstart.RestartJob(ctx, "cups_proxy"); err != nil {
		s.Fatal("Failed to restart cups_proxy: ", err)
	}

	cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "test_echo_ipp_server")
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to run test echo server: ", err)
	}
	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	ch, err := net.Dial("unix", "/run/cups_proxy/cups_proxy.sock")
	if err != nil {
		s.Fatal("Failed to open unix socket: ", err)
	}
	defer ch.Close()

	testEcho(ctx, s, ch)
	test100Continue(ctx, s, ch)
	testChunkedRequest(ctx, s, ch)
}
