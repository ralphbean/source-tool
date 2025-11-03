// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package gitlab

import (
	"testing"

	"github.com/slsa-framework/source-tool/pkg/sourcetool/models"
)

func TestBackend_CreatePipelineMR_NoBranches(t *testing.T) {
	backend := New()
	repo := &models.Repository{
		Hostname: "gitlab.com",
		Path:     "test/project",
	}

	_, err := backend.CreatePipelineMR(repo, []*models.Branch{})
	if err == nil {
		t.Fatal("Expected error when no branches specified, got nil")
	}
	if err.Error() != "no branches specified" {
		t.Errorf("Expected 'no branches specified' error, got: %v", err)
	}
}

// Integration tests would require mocking GitLab API
func TestBackend_CreatePipelineMR_Integration(t *testing.T) {
	t.Skip("Integration test requiring GitLab API access")
}
