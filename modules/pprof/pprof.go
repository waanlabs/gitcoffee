// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pprof

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"code.gitea.io/gitea/modules/log"
)

// DumpMemProfileForUsername dumps a memory profile at pprofDataPath as memprofile_<username>_<temporary id>
func DumpMemProfileForUsername(pprofDataPath, username string) error {
	f, err := os.CreateTemp(pprofDataPath, fmt.Sprintf("memprofile_%s_", username))
	if err != nil {
		return err
	}
	defer f.Close()
	runtime.GC() // get up-to-date statistics
	return pprof.WriteHeapProfile(f)
}

// DumpCPUProfileForUsername dumps a CPU profile at pprofDataPath as cpuprofile_<username>_<temporary id>
// the stop function it returns stops, writes and closes the CPU profile file
func DumpCPUProfileForUsername(pprofDataPath, username string) (func(), error) {
	f, err := os.CreateTemp(pprofDataPath, fmt.Sprintf("cpuprofile_%s_", username))
	if err != nil {
		return nil, err
	}

	err = pprof.StartCPUProfile(f)
	if err != nil {
		log.Fatal("StartCPUProfile: %v", err)
	}
	return func() {
		pprof.StopCPUProfile()
		err = f.Close()
		if err != nil {
			log.Fatal("StopCPUProfile Close: %v", err)
		}
	}, nil
}
