// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package gitlab

import (
	"strings"
	"testing"
)

func TestAppendToGitLabCI_EmptyFile(t *testing.T) {
	existingContent := ""
	includeContent := getGitLabCIInclude()

	result := appendToGitLabCI(existingContent, includeContent)

	// Should contain the include
	if !strings.Contains(result, "include:") {
		t.Error("Expected result to contain include")
	}

	// Should contain extends
	if !strings.Contains(result, "extends: .slsa-source") {
		t.Error("Expected result to contain job extending template")
	}
}

func TestAppendToGitLabCI_ExistingContent(t *testing.T) {
	existingContent := `stages:
  - build
  - test

build-job:
  stage: build
  script:
    - echo "Building"
`

	includeContent := getGitLabCIInclude()
	result := appendToGitLabCI(existingContent, includeContent)

	// Should preserve existing content
	if !strings.Contains(result, "build-job:") {
		t.Error("Expected result to preserve existing jobs")
	}

	// Should add include
	if !strings.Contains(result, "include:") {
		t.Error("Expected result to add include")
	}

	// Should add SLSA job
	if !strings.Contains(result, "slsa-source-provenance:") {
		t.Error("Expected result to add SLSA job")
	}
}

func TestAppendToGitLabCI_AlreadyHasInclude(t *testing.T) {
	existingContent := `include:
  - local: '.gitlab/slsa-source.yml'

slsa-source-provenance:
  extends: .slsa-source
`

	includeContent := getGitLabCIInclude()
	result := appendToGitLabCI(existingContent, includeContent)

	// Should not duplicate
	if strings.Count(result, "slsa-source-provenance:") > 1 {
		t.Error("Expected not to duplicate SLSA job")
	}
}
