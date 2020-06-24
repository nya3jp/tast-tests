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
	"hash"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const fakeVersionName = "20200304_22:10"

// Server is a simplestreams HTTP server that serves lxd images for tests.
// It serves images from a given directory only, and ignores the leading part of
// the path in URLs, only serving files based on matching filename.
// You should have valid lxd.tar.xz and (rootfs.squashfs or rootfs.tar.xz)
// files in the given directory.
type Server struct {
	server *http.Server
	cancel context.CancelFunc
	errs   chan (error)
}

// getIPAddress finds the externally visible IP address of localhost.
func getIPAddress() (net.IP, error) {
	// Note: we never actually send anything over this connection, the
	// destination address is irrelevant as long as it is on the external network.
	connection, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer connection.Close()

	return connection.LocalAddr().(*net.UDPAddr).IP, nil
}

// Types representing the index.json file schema.
type indexJSON struct {
	Index  indexJSONIndex `json:"index"`
	Format string         `json:"format"`
}

type indexJSONIndex struct {
	Images indexJSONImages `json:"images"`
}

type indexJSONImages struct {
	Path     string   `json:"path"`
	Datatype string   `json:"datatype"`
	Products []string `json:"products"`
}

// Types representing the images.json file schema.
type imagesJSON struct {
	Datatype  string                       `json:"datatype"`
	ContentID string                       `json:"content_id"`
	Format    string                       `json:"format"`
	Products  map[string]imagesJSONProduct `json:"products"`
}

type imagesJSONProduct struct {
	ReleaseTitle string                       `json:"release_title"`
	Versions     map[string]imagesJSONVersion `json:"versions"`
	Release      string                       `json:"release"`
	Arch         string                       `json:"arch"`
	Os           string                       `json:"os"`
	Aliases      string                       `json:"aliases"`
}

type imagesJSONVersion struct {
	Items map[string]*imagesJSONItem `json:"items"`
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

func detectArch() string {
	if runtime.GOARCH == "amd64" {
		return runtime.GOARCH
	}
	return "arm64"
}

// product returns the product name that Crostini lxd will ask for.
func product(arch string) string {
	return fmt.Sprintf("debian:buster:%s:default", arch)
}

func makeIndexJSON() ([]byte, error) {
	return json.Marshal(
		indexJSON{
			Index: indexJSONIndex{
				Images: indexJSONImages{
					Path:     "streams/v1/images.json",
					Datatype: "image-downloads",
					Products: []string{product(detectArch())},
				},
			},
			Format: "index:1.0",
		})
}

func makeImagesJSON(imageDirectory string) ([]byte, error) {
	arch := detectArch()
	items, err := makeImagesJSONItems(imageDirectory, product(arch))
	if err != nil {
		return nil, err
	}
	images := &imagesJSON{
		ContentID: "images",
		Datatype:  "image-downloads",
		Products: map[string]imagesJSONProduct{
			product(arch): imagesJSONProduct{
				Arch: arch,
				Versions: map[string]imagesJSONVersion{
					fakeVersionName: imagesJSONVersion{
						Items: items,
					},
				},
				Release:      "buster",
				Os:           "Debian",
				ReleaseTitle: "buster",
				Aliases:      "debian/buster",
			},
		},
		Format: "products:1.0",
	}
	return json.Marshal(images)
}

func makeImagesJSONItems(imageDirectory, product string) (map[string]*imagesJSONItem, error) {
	productPath := "images/" + strings.ReplaceAll(product, ":", "/") + "/" + fakeVersionName + "/"
	ftypes := map[string]string{
		"rootfs.tar.xz":   "root.tar.xz",
		"lxd.tar.xz":      "lxd.tar.xz",
		"rootfs.squashfs": "squashfs",
	}

	combinedSquashfs := ""
	combinedRootfs := ""

	items := map[string]*imagesJSONItem{}
	for filename, ftype := range ftypes {
		sha, written, err := hashFile(sha256.New(), path.Join(imageDirectory, filename))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		items[filename] = &imagesJSONItem{
			Path:   productPath + filename,
			Sha256: hex.EncodeToString(sha.Sum(nil)),
			Size:   int(written),
			Ftype:  ftype,
		}

		// Calculate combined shas.
		if filename != "lxd.tar.xz" {
			sha, _, err = hashFile(sha, path.Join(imageDirectory, "lxd.tar.xz"))
			if err != nil {
				return nil, err
			}
			if filename == "rootfs.tar.xz" {
				combinedRootfs = hex.EncodeToString(sha.Sum(nil))
			} else if filename == "rootfs.squashfs" {
				combinedSquashfs = hex.EncodeToString(sha.Sum(nil))
			}
		}
	}

	items["lxd.tar.xz"].CombinedSha256 = combinedRootfs
	items["lxd.tar.xz"].CombinedRootxzSha256 = combinedRootfs
	items["lxd.tar.xz"].CombinedSquashfsSha256 = combinedSquashfs

	return items, nil
}

func hashFile(h hash.Hash, file string) (hash.Hash, int64, error) {
	f, err := os.Open(file)
	if err != nil {
		return h, 0, err
	}
	defer f.Close()
	written, err := io.Copy(h, f)
	return h, written, err
}

// bytesHandler serves the given bytes to any HTTP requests.
func bytesHandler(ctx context.Context, bytes []byte) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(bytes); err != nil {
			testing.ContextLogf(ctx, "Error: Couldn't write file requested from image server at url %s: %v", r.URL.Path, err)
			return
		}
	}
}

// fileHandler serves files from the image directory over HTTP.
// It ignores any directory in the request path, it only matches the filename.
func fileHandler(ctx context.Context, imageDirectory string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		testing.ContextLogf(ctx, "File %s requested from tast lxd server", r.URL.Path)
		_, filename := path.Split(r.URL.Path)
		filepath := path.Join(imageDirectory, filename)
		f, err := os.Open(filepath)
		if err != nil {
			testing.ContextLogf(ctx, "Error: Couldn't open file %s requested from image server at url %s: %v", filepath, r.URL.Path, err)
		}
		defer f.Close()
		if _, err := io.Copy(w, f); err != nil {
			testing.ContextLogf(ctx, "Error: Couldn't copy file %s requested from image server at url %s: %v", filepath, r.URL.Path, err)
		}
		if err := os.RemoveAll(filepath); err != nil {
			testing.ContextLogf(ctx, "Error: Couldn't delete file %s after it was requested from image server at url %s: %v", filepath, r.URL.Path, err)
		}
	}
}

// NewServer creates a new simplestreams lxd container server
// serving images from the specified directory.
func NewServer(ctx context.Context, imageDirectory string) (*Server, error) {
	indexJSON, err := makeIndexJSON()
	if err != nil {
		return nil, err
	}

	imagesJSON, err := makeImagesJSON(imageDirectory)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/streams/v1/index.json", bytesHandler(ctx, indexJSON))
	mux.HandleFunc("/streams/v1/images.json", bytesHandler(ctx, imagesJSON))
	mux.HandleFunc("/images/", fileHandler(ctx, imageDirectory))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		testing.ContextLogf(ctx, "Tast lxd server received request to unknown path %s", r.URL)
	})

	server := &http.Server{Handler: mux}
	return &Server{server: server, errs: make(chan (error), 1)}, err
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
	go func() {
		testing.ContextLogf(ctx, "Starting LXD image server on %s", s.server.Addr)
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "Error running LXD image server: ", err)
		}
	}()

	var serverCtx context.Context
	serverCtx, s.cancel = context.WithCancel(ctx)
	go func() {
		<-serverCtx.Done()
		s.errs <- s.server.Shutdown(ctx)
	}()

	return s.server.Addr, nil
}

// Shutdown gracefully shuts down the server after all connections have completed.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.cancel == nil {
		return errors.New("cannot shutdown lxd server, it is not running")
	}
	s.cancel()
	s.cancel = nil
	return <-s.errs
}
