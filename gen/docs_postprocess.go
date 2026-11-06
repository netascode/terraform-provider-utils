// Copyright © 2022 Cisco Systems, Inc. and its affiliates.
// All rights reserved.
//
// Licensed under the Mozilla Public License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://mozilla.org/MPL/2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: MPL-2.0

//go:build ignore

package main

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
)

var docPaths = []string{"./docs/data-sources/", "./docs/functions/"}

// pageTitleRegex matches the default tfplugindocs page titles, e.g.
// "utils_yaml_merge Data Source - terraform-provider-utils" or
// "merge function - terraform-provider-utils".
var pageTitleRegex = regexp.MustCompile(`(?m)^page_title: "(\S+) (Data Source|function) - terraform-provider-utils"$`)

func main() {
	for _, path := range docPaths {
		files, _ := filepath.Glob(filepath.Join(path, "*.md"))
		for _, filename := range files {
			content, err := os.ReadFile(filename)
			if err != nil {
				log.Fatalf("Error reading file: %v", err)
			}
			s := pageTitleRegex.ReplaceAllStringFunc(string(content), func(m string) string {
				g := pageTitleRegex.FindStringSubmatch(m)
				kind := g[2]
				if kind == "function" {
					kind = "Function"
				}
				return `page_title: "` + g[1] + ` (` + kind + `)"`
			})
			if err := os.WriteFile(filename, []byte(s), 0644); err != nil {
				log.Fatalf("Error writing file: %v", err)
			}
		}
	}
}
