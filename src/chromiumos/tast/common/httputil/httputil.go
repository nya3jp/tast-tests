// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package httputil

import (
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"chromiumos/tast/errors"
)

func timeoutDialer(cTimeout, rwTimeout time.Duration) func(net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(netw, addr, cTimeout)
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(time.Now().Add(rwTimeout))
		return conn, nil
	}
}

// NewTimeoutClient returns a new dial connection with connection timeout and read/write timeout.
func NewTimeoutClient(connectTimeout, readWriteTimeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: timeoutDialer(connectTimeout, readWriteTimeout),
		},
	}
}

// HTTPGet executes http GET request and returns the body as bytes and status code.
func HTTPGet(url string, timeout time.Duration) ([]byte, int, error) {
	client := NewTimeoutClient(timeout, timeout)
	var resp *http.Response
	var err error
	var body []byte

	resp, err = client.Get(url)

	if err != nil {
		return nil, 0, errors.Wrapf(err, "Failed to invoke url %v", url)
	}

	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, resp.StatusCode, errors.Wrapf(err, "Failed to get result from url %v", url)
	}

	return body, resp.StatusCode, nil
}

// HTTPGetStr executes http GET request and returns the body as string and status code.
func HTTPGetStr(url string, timeout time.Duration) (string, int, error) {
	body, statusCode, err := HTTPGet(url, 30*time.Second)

	if err != nil {
		return "", statusCode, err
	}

	response := strings.TrimSuffix(string(body), "\n")
	return response, statusCode, nil
}
