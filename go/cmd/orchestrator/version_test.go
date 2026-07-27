/*
   Copyright 2014 Outbrain Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultAppVersionMatchesReleaseFile(t *testing.T) {
	// Walk up from this test file's package dir to the repo root.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var release string
	for {
		b, err := os.ReadFile(filepath.Join(dir, "RELEASE_VERSION"))
		if err == nil {
			release = strings.TrimSpace(string(b))
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("RELEASE_VERSION not found")
		}
		dir = parent
	}
	if release != defaultAppVersion {
		t.Fatalf("defaultAppVersion %q must match RELEASE_VERSION %q", defaultAppVersion, release)
	}
}

func TestVersionOutputNeverEmpty(t *testing.T) {
	saved := AppVersion
	defer func() { AppVersion = saved }()

	AppVersion = ""
	out := versionOutput()
	if strings.TrimSpace(out) == "" {
		t.Fatal("versionOutput empty when AppVersion unset")
	}
	if !strings.Contains(out, defaultAppVersion) {
		t.Fatalf("versionOutput %q missing defaultAppVersion", out)
	}

	AppVersion = "9.9.9"
	if versionOutput() != "9.9.9" {
		t.Fatalf("versionOutput should prefer ldflags AppVersion, got %q", versionOutput())
	}
}
