// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lxd is a fake lxd simplestreams server that serves container images for Crostini tests.
package lxd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const fakeVersionName = "20200304_22:10"

type logFunc func(format string, args ...interface{})

// Server is a simplestreams HTTP server that serves lxd images for tests.
// It serves images from a given directory only, and ignores the leading part of
// the path in URLs, only serving files based on matching filename.
// You should have a valid index.json, images.json, lxd.tar.xz, and rootfs.squashfs
// file in the given directory.
type Server struct {
	server *http.Server
	logf   logFunc
}

func getIPAddress() (net.IP, error) {
	connection, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer connection.Close()

	return connection.LocalAddr().(*net.UDPAddr).IP, nil
}

var sha256Exp = regexp.MustCompile(",\\s*\"[^\"]*sha256\"\\:\\s*\"[^\"]*\"")

type indexJSONImages struct {
	Path     string   `json:"path"`
	Datatype string   `json:"datatype"`
	Products []string `json:"products"`
}

type indexJSONIndex struct {
	Images indexJSONImages `json:"images"`
}

type indexJSON struct {
	Index  indexJSONIndex `json:"index"`
	Format string         `json:"format"`
}

type imagesJSONItem struct {
	CombinedSquashfsSha256 string `json:"combined_squashfs_sha256,omitempty"`
	Path                   string `json:"path"`
	Size                   int    `json:"size"`
	Ftype                  string `json:"ftype"`
	CombinedRootxzSha256   string `json:"combined_rootxz_sha256,omitempty"`
	CombinedSha256         string `json:"combined_sha256,omitempty"`
	Sha256                 string `json:"sha256"`
}

type imagesJSONVersion struct {
	Items map[string]*imagesJSONItem `json:"items"`
}

type imagesJSONProduct struct {
	ReleaseTitle string                       `json:"release_title"`
	Versions     map[string]imagesJSONVersion `json:"versions"`
	Release      string                       `json:"release"`
	Arch         string                       `json:"arch"`
	Os           string                       `json:"os"`
	Aliases      string                       `json:"aliases"`
}

type imagesJSON struct {
	Datatype  string                       `json:"datatype"`
	ContentID string                       `json:"content_id"`
	Format    string                       `json:"format"`
	Products  map[string]imagesJSONProduct `json:"products"`
}

func product() string {
	arch := "arm64"
	if runtime.GOARCH == "amd64" {
		arch = "amd64"
	}
	return fmt.Sprintf("debian:buster:%s:default", arch)
}

func indexJSONHandler(logf logFunc, product string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logf("index.json file %s requested from lxd server", r.URL.Path)
		index := &indexJSON{
			Index: indexJSONIndex{
				Images: indexJSONImages{
					Path:     "streams/v1/images.json",
					Datatype: "image-downloads",
					Products: []string{product},
				},
			},
			Format: "index:1.0",
		}

		bytes, err := json.Marshal(index)
		if err != nil {
			logf("Error: Unable to marshal index.json: %v", err)
			return
		}
		if _, err := w.Write(bytes); err != nil {
			logf("Error: Couldn't write index.json requested from image server at url %s: %v", r.URL.Path, err)
			return
		}
	}
}

func makeItemsForProduct(product string, items map[string]*imagesJSONItem) map[string]*imagesJSONItem {
	productPath := "images/" + strings.ReplaceAll(product, ":", "/") + "/" + fakeVersionName + "/"
	output := map[string]*imagesJSONItem{}
	for filename, item := range items {
		output[filename] = &imagesJSONItem{
			Path:                   productPath + item.Path,
			Size:                   item.Size,
			Sha256:                 item.Sha256,
			CombinedSha256:         item.CombinedSha256,
			CombinedRootxzSha256:   item.CombinedRootxzSha256,
			CombinedSquashfsSha256: item.CombinedSquashfsSha256,
			Ftype:                  item.Ftype,
		}
	}
	return output
}

func imagesJSONHandler(logf logFunc, imageDirectory string, items map[string]*imagesJSONItem, product string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logf("images.json file %s requested from lxd server", r.URL.Path)
		parts := strings.Split(product, ":")
		// aliases looks like:
		// "debian/buster/test,debian/buster/test/arm64" or
		// "debian/stretch/default,debian/stretch/default/amd64,debian/stretch,debian/stretch/amd64"
		aliases := fmt.Sprintf("debian/%s/%s,debian/%s/%s/%s", parts[1], parts[3], parts[1], parts[3], parts[2])
		if parts[3] == "default" {
			aliases = aliases + fmt.Sprintf(",debian/%s,debian/%s/%s", parts[1], parts[1], parts[2])
		}
		images := &imagesJSON{
			ContentID: "images",
			Datatype:  "image-downloads",
			Products: map[string]imagesJSONProduct{
				product: imagesJSONProduct{
					Arch: parts[2],
					Versions: map[string]imagesJSONVersion{
						fakeVersionName: imagesJSONVersion{
							Items: items,
						},
					},
					Release:      parts[1],
					Os:           "Debian",
					ReleaseTitle: parts[1],
					Aliases:      aliases,
				},
			},
			Format: "products:1.0",
		}
		bytes, err := json.Marshal(images)
		if err != nil {
			logf("Error: Unable to marshal images.json: %v", err)
			return
		}
		if _, err := w.Write(bytes); err != nil {
			logf("Error: Couldn't write images.json requested from image server at url %s: %v", r.URL.Path, err)
			return
		}
	}
}

func fileHandler(logf logFunc, imageDirectory string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logf("File %s requested from lxd server", r.URL.Path)
		c := strings.Split(r.URL.Path, "/")
		filename := path.Join(imageDirectory, c[len(c)-1])
		f, err := os.Open(filename)
		if err != nil {
			logf("Error: Couldn't open file %s requested from image server at url %s: %v", filename, r.URL.Path, err)
		}
		defer f.Close()
		if _, err := io.Copy(w, f); err != nil {
			logf("Error: Couldn't copy file %s requested from image server at url %s: %v", filename, r.URL.Path, err)
		}
	}
}

func generateItems(imageDirectory, product string) (map[string]*imagesJSONItem, error) {
	productPath := "images/" + strings.ReplaceAll(product, ":", "/") + "/" + fakeVersionName + "/"
	templates := map[string]*imagesJSONItem{
		"rootfs.tar.xz": &imagesJSONItem{
			Ftype: "root.tar.xz",
		},
		"lxd.tar.xz": &imagesJSONItem{
			Ftype: "lxd.tar.xz",
		},
		"rootfs.squashfs": &imagesJSONItem{
			Ftype: "squashfs",
		},
	}

	combinedSquashfs := ""
	combinedRootfs := ""

	items := map[string]*imagesJSONItem{}
	for filename, template := range templates {
		sha := sha256.New()
		f, err := os.Open(path.Join(imageDirectory, filename))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		defer f.Close()
		written, err := io.Copy(sha, f)
		if err != nil {
			return nil, err
		}
		items[filename] = &imagesJSONItem{
			Path:   productPath + filename,
			Sha256: hex.EncodeToString(sha.Sum([]byte{})),
			Size:   int(written),
			Ftype:  template.Ftype,
		}
		if filename != "lxd.tar.xz" {
			l, err := os.Open(path.Join(imageDirectory, filename))
			if err != nil {
				return nil, err
			}
			defer l.Close()
			if _, err := io.Copy(sha, f); err != nil {
				return nil, err
			}
			if filename == "rootfs.tar.xz" {
				combinedRootfs = hex.EncodeToString(sha.Sum([]byte{}))
			} else if filename == "rootfs.squashfs" {
				combinedSquashfs = hex.EncodeToString(sha.Sum([]byte{}))
			}
		}
	}

	items["lxd.tar.xz"].CombinedSha256 = combinedRootfs
	items["lxd.tar.xz"].CombinedRootxzSha256 = combinedRootfs
	items["lxd.tar.xz"].CombinedSquashfsSha256 = combinedSquashfs

	return items, nil
}

// NewServer creates a new simplestreams lxd container server
// serving images from the specified directory.
func NewServer(logf logFunc, imageDirectory string) (*Server, error) {
	product := product()
	logf("Generating checksums...")
	start := time.Now()
	items, err := generateItems(imageDirectory, product)
	if err != nil {
		return nil, err
	}
	logf("Generated checksums in %v seconds", time.Since(start))
	mux := http.NewServeMux()
	mux.HandleFunc("/streams/v1/index.json", indexJSONHandler(logf, product))
	mux.HandleFunc("/streams/v1/images.json", imagesJSONHandler(logf, imageDirectory, items, product))
	mux.HandleFunc("/images/", fileHandler(logf, imageDirectory))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		logf("LXD image server received request to unimplemented path %v", r.URL)
	})

	server := &http.Server{Handler: mux}
	return &Server{server: server, logf: logf}, err
}

// ListenAndServe starts the server listening in a new goroutine.
// Ensure that you call Shutdown to terminate the goroutine.
func (s *Server) ListenAndServe(ctx context.Context) (string, error) {

	// We use the port that Cicerone opens between the vm and host for gRPC.  We may need more
	// logic here if this stops working in the future.
	ip, err := getIPAddress()
	if err != nil {
		return "", err
	}
	s.server.Addr = ip.String() + ":8889"
	go (func() {
		s.logf("Starting LXD image server on %s", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil {
			if !strings.Contains(err.Error(), "Server closed") {
				s.logf("Error running LXD image server: %v", err)
				return
			}
		}
	})()
	return s.server.Addr, nil
}

// Shutdown gracefully shuts down the server after all connections have completed.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
