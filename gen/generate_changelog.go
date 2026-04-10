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
