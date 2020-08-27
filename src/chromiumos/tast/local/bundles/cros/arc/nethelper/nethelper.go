// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nethelper provides functionality to support test execution by handling
// requests from various tests coming via network in context of ARC TAST test.
// arc_eth0 on port 1235 is used as communicatin point. This helper supports
// following commands.
//   * drop_caches - drops system caches, returns OK/FAILED
//
// Usage pattern is following:
// 	conn, err := nethelper.Start(ctx)
//	if err != nil {
//		s.Fatal("Failed to start helper", err)
//	}
//	defer conn.Close()
package nethelper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Connection describes running socket server.
type Connection struct {
	// Contains network address that could be used by client to connect this server
	Address string
	listen  net.Listener
}

// Close closes the connection
func (c *Connection) Close() {
	c.listen.Close()
}

// Start starts socket server and returns connection descriptor.
func Start(ctx context.Context) (*Connection, error) {
	const (
		port = 1235
	)

	// Make sure ports may accepts connection.
	if err := testexec.CommandContext(ctx,
		"/sbin/iptables",
		"-A", "INPUT", "-p", "tcp",
		"--dport", strconv.Itoa(port),
		"-j", "ACCEPT").Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to open port")
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, errors.Wrap(err, "failed to enum interfaces")
	}

	for _, i := range ifaces {
		// Handle real addresses that could be accessible for the current DUT.
		if i.Name != "arc_eth0" {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			testing.ContextLogf(ctx, "Failed to enum addresses for %s", i.Name)
			continue
		}
		if len(addrs) == 0 {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				result := new(Connection)
				if v.IP.To4() != nil {
					result.Address = fmt.Sprintf("%s:%d", v.IP, port)
				} else {
					result.Address = fmt.Sprintf("[%s%s%s]:%d", v.IP, "%", i.Name, port)
				}
				result.listen, err = net.Listen("tcp", result.Address)
				if err != nil {
					testing.ContextLogf(ctx, "Failed to listen on %s", result.Address)
					continue
				}
				testing.ContextLogf(ctx, "Listening on %s", result.Address)
				go listenForClients(ctx, result.listen)
				return result, nil
			}
		}

	}
	return nil, errors.New("failed to start server, no address is availables")
}

func listenForClients(ctx context.Context, listen net.Listener) {
	for {
		conn, err := listen.Accept()
		if err != nil {
			testing.ContextLogf(ctx, "Stop listening %s", err)
			return
		}
		testing.ContextLogf(ctx, "Connection is ready %s", conn.RemoteAddr().String())
		go handleClient(ctx, conn)
	}

}

func respond(ctx context.Context, conn net.Conn, result string) bool {
	_, err := conn.Write([]byte(result + "\n"))
	if err != nil {
		testing.ContextLogf(ctx, "Failed to respond %q to %s", result, conn.RemoteAddr().String())
		return false
	}

	return true
}

func handleClient(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	const (
		cmdDropCaches = "drop_caches"
	)

	for {
		message, err := bufio.NewReader(conn).ReadString('\n')
		switch err {
		case io.EOF:
			testing.ContextLogf(ctx, "Connection is closed %s", conn.RemoteAddr().String())
			return
		case nil:
			msg := strings.TrimSuffix(string(message), "\n")
			switch msg {
			case cmdDropCaches:
				if !respond(ctx, conn, handleDropCaches(ctx)) {
					return
				}
			default:
				testing.ContextLogf(ctx, "Unknown command %s: %s", conn.RemoteAddr().String(), msg)
			}
		default:
			testing.ContextLogf(ctx, "Connection is broken %s: %s", conn.RemoteAddr().String(), err)
			return
		}
	}
}

func handleDropCaches(ctx context.Context) string {
	if err := testexec.CommandContext(ctx, "sync").Run(testexec.DumpLogOnError); err != nil {
		testing.ContextLogf(ctx, "Failed to flush buffers: %s", err)
		return "FAILED"
	}
	if err := ioutil.WriteFile("/proc/sys/vm/drop_caches", []byte("3"), 0200); err != nil {
		testing.ContextLogf(ctx, "Failed to drop caches: %s", err)
		return "FAILED"
	}
	testing.ContextLog(ctx, "Cleared caches, system buffer, dentries and inodes")
	return "OK"
}
