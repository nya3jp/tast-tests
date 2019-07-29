#!/usr/bin/env python2
# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""A simple UDP socket server run in guest container to communicate to host.

It waits for 2 types of events:
  - keyboard events routed by Wayland to stdin.
  - socket messages from host (e.g., from CrostiniInputLatency test).
When any event comes in, returns a response to client immediately.
"""
from __future__ import print_function
import socket
import select
import sys

def Loop(sock):
  server_addr = None
  while True:
    # non-blocking
    readable, _, _ = select.select([sys.stdin, sock], [], [], 0.0)
    for fd in readable:
      if fd is sys.stdin:
        if not server_addr:
          print("Unknown server address, unable to send key event response")
          continue
        sock.sendto("keyEvent", server_addr)
        data = sys.stdin.readline()
        if not data:
            return
      elif fd is sock:
        # Update |server_addr| by latest ping address.
        data, server_addr = sock.recvfrom(1024)
        if data == "ping":
          sock.sendto("pong", server_addr)
        elif not data or data == "exit":
          sock.sendto("exit", server_addr)
          return
        else:
          print("Unrecognizable command", data)


def main():
  PORT_FILENAME = "crostini_socket_server_port"

  sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
  sock.bind(("", 0))
  port = sock.getsockname()[1]
  with open(PORT_FILENAME, "w") as port_file:
    port_file.write(str(port))
  print("port", port)

  Loop(sock)
  print("exit")

main()
