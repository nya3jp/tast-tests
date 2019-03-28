// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"io/ioutil"
	"net"
	"os"

	"testing"
)

func TestNewServodProxy(t *testing.T) {
	/* DEBUGGING ------------

	This test file is currently a disaster as I figure out how to properly test
	reading/writing through ServodProxy.
	*/


	ctx := context.TODO()
	remotePipe, remotePipeOut := net.Pipe()
	expected := []byte("rutabaga")
	cf := func(context.Context) (net.Conn, error) {
		return remotePipe, nil
	}
	chDone := make(chan bool)

	go func() {
		os.Stderr.WriteString("IN TEST------\n")
		sdp, err := NewServodProxy(ctx, cf)
		if err != nil {
			t.Fatal(err)
		}
		localTCP, err := net.Dial("tcp", sdp.LocalAddress())
		if err != nil {
			t.Fatal(err)
		}

		bytesWritten, err := localTCP.Write(expected)
		if err != nil {
			t.Fatal(err)
		}
		/*
		bytesWritten, err := remotePipe.Write(expected)
		if err != nil {
			t.Fatal(err)
		}
		*/
		if bytesWritten != len(expected) {
			t.Errorf("conn wrote %d bytes, expected %d", bytesWritten, len(expected))
		}
		if remotePipe.Close(); err != nil {
			t.Fatal(err)
		}
		/*
		if localTCP.Close(); err != nil {
			t.Fatal(err)
		}
		*/
		if sdp.Close(); err != nil {
			t.Fatal(err)
		}
		chDone <- true
	}()

	go func() {
		os.Stderr.WriteString("READING------\n")
		actual, err := ioutil.ReadAll(remotePipeOut)
		if err != nil {
			t.Fatalf("reading remote conn failed: %v", err)
		}
		os.Stderr.WriteString("GOT: ")
		os.Stderr.WriteString(string(actual))
		os.Stderr.WriteString("\n")
	}()

	<-chDone
	t.Fatalf("ending to print...")
}

/*
func TestHandleProxyConnection(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()

	go handleProxyConnection(local, remote)

	expected := []byte("rutabaga")
	go func() {
		local.Write(expected)
		local.Close()
	}()

	actual, err := ioutil.ReadAll(remote)
	if err != nil {
		t.Fatalf("reading remote conn failed: %v", err)
	}

	actualStr := string(actual)
	expectedStr := string(expected)
	if actualStr != expectedStr {
		t.Errorf("proxyConnection() with input %q copied %q; want %q", expectedStr, actualStr, expectedStr)
	}
}
*/

































	//pipeLeft, pipeRight := net.Pipe()
	//defer local.Close()
	//defer remote.Close()

	//ctx := context.TODO()

	// Set up a "remote" TCP listener.
	//remoteTCP, err := net.Dial("tcp", string(remoteListener.Addr()))
	//remoteTCP, err := net.Dial("tcp", ":0")
	//defer remoteTCP.Close()
	/*
	if err != nil {
		t.Fatal(err)
	}

	cf := func(context.Context) (net.Conn, error) {
		return remoteTCP, nil
	}

	//sdp, err := NewServodProxy(ctx, cf)
	_, err = NewServodProxy(ctx, cf)
	if err != nil {
		t.Fatal(err)
	}
	*/



	//chDone := make(chan bool)
	//chDone2 := make(chan bool)
	/*
	var actual []byte
	go func() {
		_, err := pipeRight.Read(actual)
		*actual, err = ioutil.ReadAll(pipeRight)
		if err != nil {
			t.Fatalf("reading remote conn failed: %v", err)
		}

		chDone <- true
	}()

	<-chDone
	/*
	//go func(proxy *ServodProxy, lc net.Conn) {
	go func(sdp *ServodProxy, t *testing.T) {
		t.Fatalf("wat2")
		localTCP, err := net.Dial("tcp", sdp.LocalAddress())
		if err != nil {
			t.Fatal(err)
		}
		bytesWritten, err := localTCP.Write(expected)
		if err != nil {
			t.Fatal(err)
		}


		if localTCP.Close(); err != nil {
			t.Fatal(err)
		}
		if pipeLeft.Close(); err != nil {
			t.Fatal(err)
		}
		chDone2 <- true
	}(sdp, t)
	go func(pipeLeft net.Conn, t *testing.T) {
		if _, err := pipeLeft.Write(expected); err != nil {
			t.Fatal(err)
		}
		if err := pipeLeft.Close(); err != nil {
			t.Fatal(err)
		}
		/*
		if err := sdp.Close(); err != nil {
			t.Fatal(err)
		}
	}(pipeLeft, t)

	*/

	//<-chDone2



	/*
	var actual []byte
	go func(actualLoc *[]byte) {
		*actualLoc, err = ioutil.ReadAll(local)
		if err != nil {
			t.Fatalf("reading remote conn failed: %v", err)
		}
		if remote.Close(); err != nil {
			t.Fatal(err)
		}
		chDone2 <- true
	}(&actual)
	*/


	//}(sdp, local)

	//<-chDone2
	/*
	actualStr := string(actual)
	expectedStr := string(expected)
	if actualStr != expectedStr {
		t.Errorf("ServodProxy wrote %q; want %q", actualStr, expectedStr)
	}
	*/
