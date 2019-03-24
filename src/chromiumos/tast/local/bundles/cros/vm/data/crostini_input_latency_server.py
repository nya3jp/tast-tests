#!/usr/bin/env python2
# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""A simple TCP socket server run in guest container to communicate to host.

It waits for 2 types of events:
  - keyboard events routed by Wayland to stdin.
  - messages from host (e.g., CrostiniInputLatency test) via the TCP socket.
When any event comes in, returns local timestamp in response.
"""
import errno
import os
import socket
import select
import sys
import time

def Loop(conn):
  while True:
    # non-blocking
    readable, _, _, = select.select([sys.stdin, conn], [], [], 0.0)
    for fd in readable:
      if fd is sys.stdin:
        t = time.time()
        conn.sendall("%.9f\n" % t)
        data = sys.stdin.readline()
        print 'key ', t
        if not data:
            return
      elif fd is conn:
        data = conn.recv(1024)
        if data == "ping":
          t = time.time()
          conn.sendall("%.9f\n" % t)
          print 'ping', t
        elif not data or data == "exit":
          print "exit"
          return
        else:
          print "unrecognizable command", data


def RemoveFileIfExist(filename):
    try:
        os.remove(filename)
    except OSError as e:
        if e.errno != errno.ENOENT:
            raise

def main():
  PORT_FILENAME = "crostini_socket_server_port"

  s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
  s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)

  RemoveFileIfExist(PORT_FILENAME)
  s.bind(("", 0))
  port = s.getsockname()[1]
  with open(PORT_FILENAME, "w") as port_file:
    port_file.write(str(port))
  print "port", port

  s.listen(1)
  conn, addr = s.accept()
  print 'Connected by', addr

  Loop(conn)
  conn.close()
  s.close()

main()
