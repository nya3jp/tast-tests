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
// You should have valid lxd.tar.xz and rootfs.squashfs files in the given
// directory.
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
	Path                   string `json:"path"`
	Size                   int    `json:"size"`
	Ftype                  string `json:"ftype"`
	CombinedSquashFsSha256 string `json:"combined_squashfs_sha256,omitempty"`
	Sha256                 string `json:"sha256"`
}

func detectArch() string {
	if runtime.GOARCH == "amd64" {
		return runtime.GOARCH
	}
	return "arm64"
}

// product returns the product name that Crostini lxd will ask for.
func product(debrel, arch string) string {
	return fmt.Sprintf("debian:%s:%s:default", debrel, arch)
}

func makeIndexJSON() ([]byte, error) {
	arch := detectArch()
	return json.Marshal(
		&indexJSON{
			Index: indexJSONIndex{
				Images: indexJSONImages{
					Path:     "streams/v1/images.json",
					Datatype: "image-downloads",
					Products: []string{product("buster", arch), product("bullseye", arch)},
				},
			},
			Format: "index:1.0",
		})
}

func makeImagesJSON(metadataPath, rootfsPath string) ([]byte, error) {
	arch := detectArch()
	items, err := makeImagesJSONItems(metadataPath, rootfsPath, product("buster", arch))
	if err != nil {
		return nil, err
	}
	images := &imagesJSON{
		ContentID: "images",
		Datatype:  "image-downloads",
		Products: map[string]imagesJSONProduct{
			product("buster", arch): {
				Arch: arch,
				Versions: map[string]imagesJSONVersion{
					fakeVersionName: {
						Items: items,
					},
				},
				Release:      "buster",
				Os:           "Debian",
				ReleaseTitle: "buster",
				Aliases:      "debian/buster,debian/bullseye",
			},
		},
		Format: "products:1.0",
	}
	return json.Marshal(images)
}

func makeImagesJSONItems(metadataPath, rootfsPath, product string) (map[string]*imagesJSONItem, error) {
	productPath := "images/" + strings.ReplaceAll(product, ":", "/") + "/" + fakeVersionName + "/"

	metadataSha, metadataSize, err := hashFile(sha256.New(), metadataPath)
	if err != nil {
		return nil, err
	}

	squashfsSha, squashfsSize, err := hashFile(sha256.New(), rootfsPath)
	if err != nil {
		return nil, err
	}

	combinedSquashfsSha, _, err := hashFile(sha256.New(), metadataPath)
	if err != nil {
		return nil, err
	}
	_, _, err = hashFile(combinedSquashfsSha, rootfsPath)
	if err != nil {
		return nil, err
	}

	items := map[string]*imagesJSONItem{}

	items["lxd.tar.xz"] = &imagesJSONItem{
		Path:                   productPath + "lxd.tar.xz",
		Size:                   int(metadataSize),
		Ftype:                  "lxd.tar.xz",
		Sha256:                 hex.EncodeToString(metadataSha.Sum(nil)),
		CombinedSquashFsSha256: hex.EncodeToString(combinedSquashfsSha.Sum(nil)),
	}

	items["root.squashfs"] = &imagesJSONItem{
		Path:   productPath + "rootfs.squashfs",
		Size:   int(squashfsSize),
		Ftype:  "squashfs",
		Sha256: hex.EncodeToString(squashfsSha.Sum(nil)),
	}

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

// flushResponse is required at the end of writing, to behave correctly as a streaming server.
func flushResponse(w http.ResponseWriter) {
	if fl, ok := w.(http.Flusher); ok {
		fl.Flush()
	}
}

// bytesHandler serves the given bytes to any HTTP requests.
func bytesHandler(ctx context.Context, bytes []byte) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer flushResponse(w)
		if _, err := w.Write(bytes); err != nil {
			testing.ContextLogf(ctx, "Error: Couldn't write file requested from image server at url %s: %v", r.URL.Path, err)
			return
		}
	}
}

// fileHandler serves files from the image directory over HTTP.
// It ignores any directory in the request path, it only matches the filename.
func fileHandler(ctx context.Context, metadataPath, rootfsPath string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer flushResponse(w)
		testing.ContextLogf(ctx, "File %s requested from tast lxd server", r.URL.Path)
		_, filename := path.Split(r.URL.Path)
		filepath := ""
		if filename == "lxd.tar.xz" {
			filepath = metadataPath
		} else if filename == "rootfs.squashfs" {
			filepath = rootfsPath
		} else {
			testing.ContextLogf(ctx, "Error: Image server got unexpected request at %s", r.URL.Path)
			return
		}

		f, err := os.Open(filepath)
		if err != nil {
			testing.ContextLogf(ctx, "Error: Couldn't open file %s requested from image server at url %s: %v", filepath, r.URL.Path, err)
		}
		defer f.Close()
		if _, err := io.Copy(w, f); err != nil {
			testing.ContextLogf(ctx, "Error: Couldn't copy file %s requested from image server at url %s: %v", filepath, r.URL.Path, err)
		}
	}
}

// NewServer creates a new simplestreams lxd container server
// serving images from the specified directory.
func NewServer(ctx context.Context, metadataPath, rootfsPath string) (*Server, error) {
	indexJSON, err := makeIndexJSON()
	if err != nil {
		return nil, err
	}

	imagesJSON, err := makeImagesJSON(metadataPath, rootfsPath)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/streams/v1/index.json", bytesHandler(ctx, indexJSON))
	mux.HandleFunc("/streams/v1/images.json", bytesHandler(ctx, imagesJSON))
	mux.HandleFunc("/images/", fileHandler(ctx, metadataPath, rootfsPath))
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
