// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package isolatedapp

import (
	"io/ioutil"
	"net/http"
)

/*
The HTTPSServer structure that stores all information to start the https server.
*/
type HTTPSServer struct {
	Headers               map[string]string
	ServerKeyPath         string
	ServerCertificatePath string
	HostedFilesBasePath   string
}

var serverConfiguration HTTPSServer

/*
StartServer starts up an https server without blocking.
*/
func StartServer(configuration HTTPSServer) {

	serverConfiguration = configuration
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		for key, value := range serverConfiguration.Headers {
			w.Header().Set(key, value)
		}
		path := r.URL.String()
		b, _ := ioutil.ReadFile(serverConfiguration.HostedFilesBasePath + "/" + path[1:len(path)])
		w.WriteHeader(200)
		w.Write([]byte(b))
	})

	go http.ListenAndServeTLS(":443", serverConfiguration.ServerCertificatePath, serverConfiguration.ServerKeyPath, nil)

}
