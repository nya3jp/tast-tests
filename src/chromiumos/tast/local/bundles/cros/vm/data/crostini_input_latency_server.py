# Copyright 2018 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""A simple socket server to communicate to CrostiniInputLatency test.

It waits for 2 types of events:
  - keyboard events routed by Wayland to stdin.
  - messages from CrostiniInputLatency test via a TCP connection.
When any event comes in, returns local timestmap in response.
"""
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

def main():
  HOST = ''
  PORT = 12346

  s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
  s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
  s.bind((HOST, PORT))
  s.listen(1)
  conn, addr = s.accept()
  print('Connected by', addr)

  Loop(conn)
  conn.close()
  s.close()

main()
