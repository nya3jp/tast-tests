// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vimcompile provides functionality to compile vim package using crostini containers and outputs the time taken to compile it
package vimcompile

import (
	"context"
	"time"

	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// RunTest compiles vim multiple times and captures the average amount of time taken to compile it.
func RunTest(ctx context.Context, s *testing.State, cont *vm.Container) {
	const (
		numberOfIterations = 5 // numberOfIterations is set to the number of times vim is to be compiled.
		configureVim = "cd /home/testuser/vim/src && ./configure"
		makeVim = "cd /home/testuser/vim/src && make -j > /dev/null"
		removeVim = "cd /home/testuser && rm -rf vim"
		untarVim = "cd /home/testuser && tar -xvf vim.tar.gz"
	)

	var collectTime time.Duration;
	i := 0

	setupTest(ctx, s, cont)

	s.Logf("Compiling vim %d times",numberOfIterations)
	for i < numberOfIterations {
		// Configuring vim package.
		s.Log("Configuring vim")
		if err := cont.Command(ctx, "sh","-c",configureVim).Run(testexec.DumpLogOnError); err != nil {
			s.Error("Failed to configure vim package: ", err)
		}

		// Compiling vim package.
		s.Logf("Iteration number %d to make vim package is in progress...",i+1)
		start := time.Now()
		if err := cont.Command(ctx, "sh","-c",makeVim).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to make vim package: ", err)
		}
		endTime := time.Since(start)
		s.Logf("Amount of time taken to compile vim in iteration %d: %fs",i+1,float64(endTime.Seconds()))
		collectTime += endTime

		// Removing vim directory.
		s.Log("Removing vim directory")
		if err := cont.Command(ctx, "sh","-c",removeVim).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to remove vim package: ", err)
		}

		// Untaring vim packge for subsequent iterations.
		s.Log("Untaring vim package")
		if err := cont.Command(ctx, "sh","-c",untarVim).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to untar vim package: ", err)
		}
		i++
	}

	// Calculating average time to compile vim.
	avgTime := collectTime/numberOfIterations
	s.Logf("Average time to compile vim: %fs",float64(avgTime.Seconds()))

	// Saving data to Chrome Performance Dashboard - https://chromeperf.appspot.com/.
	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name: "compile_time",
		Unit: "seconds",
		Direction: perf.SmallerIsBetter,
	}, float64(avgTime.Seconds()))
	if err:= pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save average compile time due to: ",err)
	}
}

// setupTest sets up the device to compile vim.
func setupTest(ctx context.Context, s *testing.State, cont *vm.Container) {
	const (
		shaValue = "fbbd10" // shaValue is from vim github where we are checking out.
		installLibs = "sudo apt-get install -y gcc make libncurses5-dev libncursesw5-dev"
		fixMissingLibs = "sudo apt-get update --fix-missing"
		cloneVim = "git clone https://github.com/vim/vim.git"
		checkoutVimSha = "cd /home/testuser/vim/src && git checkout "+shaValue
		tarVim = "tar -czvf vim.tar.gz vim"
	)

	s.Log("Installing required packages to compile vim")
	if err := cont.Command(ctx, "sh","-c",installLibs).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to install packages: ", err)
	}

	s.Log("Installing missing dependencies")
	if err := cont.Command(ctx, "sh","-c",fixMissingLibs).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to update dependencies: ", err)
	}

	s.Log("Cloning vim from github")
	if err := cont.Command(ctx, "sh","-c",cloneVim).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to clone vim package: ", err)
	}

	// This is required to ensure we measure the same content each time.
	s.Log("Checking out a specific tag in vim")
	if err := cont.Command(ctx, "sh","-c",checkoutVimSha).Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to check out a specific tag due to: ", err)
	}

	s.Log("Compressing and saving the vim folder")
	if err := cont.Command(ctx, "sh","-c",tarVim).Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to compress vim folder: ", err)
	}
}