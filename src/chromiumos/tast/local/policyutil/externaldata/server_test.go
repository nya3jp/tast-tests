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

	eds, err := NewServer(ctx)
	if err != nil {
		t.Fatal("Failed to create server: ", err)
	}
	defer eds.Stop(ctx)

	testData := []byte{1, 2, 3}
	const expectedHash = "039058c6f2c0cb492c533b0a4d14ef77cc0f78abccced5287d84a1a2011cfb81"

	url, hash := eds.ServePolicyData(testData)

	if hash != expectedHash {
		t.Errorf("Unexpected hash: want %s; got %s", expectedHash, hash)
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

func TestMissingURL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eds, err := NewServer(ctx)
	if err != nil {
		t.Fatal("Failed to create server: ", err)
	}
	defer eds.Stop(ctx)

	url, _ := eds.ServePolicyData([]byte{1, 2, 3, 4})

	resp, err := http.Get(url + "123")
	if err != nil {
		t.Fatalf("Failed to get %q: %v", url, err)
	}
	defer resp.Body.Close()
	if _, err := ioutil.ReadAll(resp.Body); err != nil {
		t.Fatal("Failed to read response body: ", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Unexpected status: got %d; want %d", resp.StatusCode, http.StatusNotFound)
	}
}
