// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package gitlab

import (
	"strings"
	"testing"
)

func TestOIDCPipelineTemplate(t *testing.T) {
	template := getOIDCPipelineTemplate()

	// Should contain OIDC configuration
	if !strings.Contains(template, "id_tokens:") {
		t.Error("Expected OIDC template to contain id_tokens")
	}

	// Should contain SIGSTORE_ID_TOKEN
	if !strings.Contains(template, "SIGSTORE_ID_TOKEN:") {
		t.Error("Expected OIDC template to contain SIGSTORE_ID_TOKEN")
	}

	// Should contain sigstore audience
	if !strings.Contains(template, "aud: sigstore") {
		t.Error("Expected OIDC template to have sigstore audience")
	}

	// Should handle both branches and tags
	if !strings.Contains(template, "CI_COMMIT_TAG") {
		t.Error("Expected template to handle tags")
	}

	if !strings.Contains(template, "checklevelprov") {
		t.Error("Expected template to handle branches")
	}
}

func TestCosignPipelineTemplate(t *testing.T) {
	template := getCosignPipelineTemplate()

	// Should NOT contain id_tokens
	if strings.Contains(template, "id_tokens:") {
		t.Error("Expected cosign template to NOT contain id_tokens")
	}

	// Should contain COSIGN_PRIVATE_KEY reference
	if !strings.Contains(template, "COSIGN_PRIVATE_KEY") {
		t.Error("Expected cosign template to reference COSIGN_PRIVATE_KEY")
	}

	// Should install cosign
	if !strings.Contains(template, "cosign") {
		t.Error("Expected cosign template to install cosign")
	}

	// Should use unsigned bundle
	if !strings.Contains(template, "output_unsigned_bundle") {
		t.Error("Expected cosign template to use unsigned bundle")
	}
}

func TestGetGitLabCIInclude(t *testing.T) {
	include := getGitLabCIInclude()

	// Should include local reference
	if !strings.Contains(include, "include:") {
		t.Error("Expected include section")
	}

	if !strings.Contains(include, "local: '.gitlab/slsa-source.yml'") {
		t.Error("Expected local reference to slsa-source.yml")
	}

	// Should have job that extends template
	if !strings.Contains(include, "extends: .slsa-source") {
		t.Error("Expected job extending .slsa-source")
	}
}
