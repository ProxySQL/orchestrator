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

package http

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestBundledJQueryMeetsSecurityFloor guards against regressing the bundled
// jQuery below 3.5.0 (CVE scanners / customer security requirements).
func TestBundledJQueryMeetsSecurityFloor(t *testing.T) {
	chdirToRepoRoot(t)

	f, err := os.Open(filepath.Join("resources", "public", "js", "jquery.min.js"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	line, err := bufio.NewReader(f).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	// e.g. /*! jQuery v3.7.1 | (c) OpenJS Foundation ...
	re := regexp.MustCompile(`jQuery v(\d+)\.(\d+)\.(\d+)`)
	m := re.FindStringSubmatch(line)
	if m == nil {
		t.Fatalf("could not parse jQuery version from header: %q", strings.TrimSpace(line))
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])

	// Floor: 3.5.0
	if major < 3 || (major == 3 && minor < 5) {
		t.Fatalf("bundled jQuery %d.%d.%d is below required 3.5.0", major, minor, patch)
	}
	t.Logf("bundled jQuery %d.%d.%d", major, minor, patch)
}
