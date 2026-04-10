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
	"text/template"
)

const (
	changelogSource   = "./CHANGELOG.md"
	changelogTemplate = "./gen/templates/changelog.md.tmpl"
	changelogOutput   = "./templates/guides/changelog.md.tmpl"
)

func main() {
	changelog, err := os.ReadFile(changelogSource)
	if err != nil {
		log.Fatalf("Error reading changelog: %v", err)
	}

	tmpl, err := template.ParseFiles(changelogTemplate)
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(changelogOutput), 0755); err != nil {
		log.Fatalf("Error creating output directory: %v", err)
	}

	f, err := os.Create(changelogOutput)
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, string(changelog)); err != nil {
		log.Fatalf("Error executing template: %v", err)
	}
}
