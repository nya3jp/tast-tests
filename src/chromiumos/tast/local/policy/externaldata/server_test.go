// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package externaldata

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestServeURL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eds, err := NewExternalDataServer(ctx, "localhost", 12345)
	if err != nil {
		t.Fatal("Failed to create server: ", err)
	}

	testData := []byte{1, 2, 3}

	url, _, err := eds.ServePolicyData(testData)
	if err != nil {
		t.Fatal("Failed to serve: ", err)
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to get %q: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Failed to read response body: ", err)
	}

	if bytes.Compare(testData, body) != 0 {
		t.Errorf("Unexpected response: want %q; got %q", testData, body)
	}
}
