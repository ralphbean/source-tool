// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package gitlab

import (
	"strings"
)

// appendToGitLabCI appends the SLSA source include to existing GitLab CI config
// without duplicating if it already exists
func appendToGitLabCI(existingContent, includeContent string) string {
	// Check if already contains SLSA source configuration
	if strings.Contains(existingContent, "slsa-source-provenance:") {
		return existingContent
	}

	// If empty, just return the include
	if strings.TrimSpace(existingContent) == "" {
		return includeContent
	}

	// Append to existing content
	return existingContent + "\n" + includeContent
}
