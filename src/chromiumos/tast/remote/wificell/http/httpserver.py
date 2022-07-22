# Lint as: python2, python3
# Copyright 2022 The ChromiumOS Authors.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

class RequestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        message = "HTTP server is running"
        self.protocol_version = "HTTP/1.1"
        self.send_response(int(sys.argv[2]))
        self.send_header("Location", sys.argv[3])
        self.end_headers()
        self.wfile.write(bytes(message, "utf8"))
        return

    def do_HEAD(self):
        self.protocol_version = "HTTP/1.1"
        self.send_response(int(sys.argv[2]))
        self.send_header("Location", sys.argv[3])
        self.end_headers()
        return

def run():
    server = ('', int(sys.argv[1]))
    httpd = HTTPServer(server, RequestHandler)
    httpd.serve_forever()

run()