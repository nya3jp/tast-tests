// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils used to do some component excution function
package utils

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"chromiumos/tast/testing"
)

var (
	wwcbIP   = "192.168.1.168"
	user     = "admin"
	password = "12345678"
)

// OpenIppower is for open ip power
func OpenIppower(ctx context.Context, ports []int) error {
	myport := ""
	for _, port := range ports {
		if port < 1 || port > 4 {
			log.Fatal("the port format error.")
		} else {
			s := fmt.Sprintf("+p6%v=1", port)
			myport += s
		}
	}

	url := fmt.Sprintf("http://%s/set.cmd?user=%s+pass=%s+cmd=setpower%s", wwcbIP, user, password, myport)
	testing.ContextLogf(ctx, "request: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		s := fmt.Sprintf("failed to send request: %s", err)
		return error.wrap(err, s)
	}
	defer resp.Body.Close()
}

// CloseIppower is for close ip power
func CloseIppower(ctx context.Context, ports []int) {
	myport := ""
	for _, port := range ports {
		if port < 1 || port > 4 {
			log.Fatal("the port format error.")
		} else {
			s := fmt.Sprintf("+p6%v=0", port)
			myport += s
		}
	}

	url := fmt.Sprintf("http://%s/set.cmd?user=%s+pass=%s+cmd=setpower%s", wwcbIP, user, password, myport)
	testing.ContextLogf(ctx, "request: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		s := fmt.Sprintf("failed to send request: %s", err)
		return error.wrap(err, s)
	}
	defer resp.Body.Close()
}
