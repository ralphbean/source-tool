// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package gitlab

import (
	"context"
	"strings"
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

func TestBackend_CreatePipelineMR_NilRepository(t *testing.T) {
	backend := New()
	branches := []*models.Branch{
		{Name: "main"},
	}

	_, err := backend.CreatePipelineMR(nil, branches)
	if err == nil {
		t.Fatal("Expected error when repository is nil, got nil")
	}
	if !strings.Contains(err.Error(), "repository is nil") {
		t.Errorf("Expected error about nil repository, got: %v", err)
	}
}

func TestBackend_CreatePipelineMR_EmptyRepositoryPath(t *testing.T) {
	backend := New()
	repo := &models.Repository{
		Hostname: "gitlab.com",
		Path:     "",
	}
	branches := []*models.Branch{
		{Name: "main"},
	}

	_, err := backend.CreatePipelineMR(repo, branches)
	if err == nil {
		t.Fatal("Expected error when repository path is empty, got nil")
	}
	if !strings.Contains(err.Error(), "repository path not set") {
		t.Errorf("Expected error about empty repository path, got: %v", err)
	}
}

func TestBackend_FindPipelineMR_NilRepository(t *testing.T) {
	backend := New()
	ctx := context.Background()

	_, err := backend.FindPipelineMR(ctx, nil)
	if err == nil {
		t.Fatal("Expected error when repository is nil, got nil")
	}
	if !strings.Contains(err.Error(), "repository is nil") {
		t.Errorf("Expected error about nil repository, got: %v", err)
	}
}

func TestBackend_FindPipelineMR_EmptyRepositoryPath(t *testing.T) {
	backend := New()
	ctx := context.Background()
	repo := &models.Repository{
		Hostname: "gitlab.com",
		Path:     "",
	}

	_, err := backend.FindPipelineMR(ctx, repo)
	if err == nil {
		t.Fatal("Expected error when repository path is empty, got nil")
	}
	if !strings.Contains(err.Error(), "repository path not set") {
		t.Errorf("Expected error about empty repository path, got: %v", err)
	}
}

// Integration tests would require mocking GitLab API
func TestBackend_CreatePipelineMR_Integration(t *testing.T) {
	t.Skip("Integration test requiring GitLab API access")
	// This would test the full flow:
	// - Check OIDC support via GitLab version
	// - Create feature branch
	// - Create .gitlab/slsa-source.yml with appropriate template
	// - Read/update .gitlab-ci.yml
	// - Create commit
	// - Create merge request
	// - Verify merge request was created with correct title/description
}

func TestBackend_FindPipelineMR_Integration(t *testing.T) {
	t.Skip("Integration test requiring GitLab API access")
	// This would test:
	// - Listing open merge requests
	// - Filtering by pipeline commit message
	// - Returning nil when no matching MR exists
	// - Returning PullRequest model when match found
}

func TestBackend_CreatePipelineMR_OIDC_Integration(t *testing.T) {
	t.Skip("Integration test requiring GitLab API access")
	// This would test:
	// - Detection of OIDC support (GitLab 15.7+)
	// - Selection of OIDC pipeline template
	// - Verification that template includes id_tokens configuration
}

func TestBackend_CreatePipelineMR_Cosign_Integration(t *testing.T) {
	t.Skip("Integration test requiring GitLab API access")
	// This would test:
	// - Detection of no OIDC support (GitLab < 15.7)
	// - Selection of Cosign pipeline template
	// - Verification that template includes COSIGN_PRIVATE_KEY reference
}

func TestBackend_CreatePipelineMR_ExistingGitLabCI_Integration(t *testing.T) {
	t.Skip("Integration test requiring GitLab API access")
	// This would test:
	// - Reading existing .gitlab-ci.yml
	// - Appending include without overwriting existing content
	// - Using FileUpdate action instead of FileCreate
}

func TestBackend_CreatePipelineMR_NoGitLabCI_Integration(t *testing.T) {
	t.Skip("Integration test requiring GitLab API access")
	// This would test:
	// - Creating new .gitlab-ci.yml when none exists
	// - Using FileCreate action
	// - Verifying content includes both include and job definition
}

func TestBackend_CreatePipelineMR_BranchAlreadyExists_Integration(t *testing.T) {
	t.Skip("Integration test requiring GitLab API access")
	// This would test:
	// - Handling case where feature branch already exists
	// - Verifying error is handled gracefully
	// - Ensuring commit is added to existing branch
}

func TestBackend_FindPipelineMR_MultipleOpenMRs_Integration(t *testing.T) {
	t.Skip("Integration test requiring GitLab API access")
	// This would test:
	// - Searching through multiple open MRs
	// - Finding the correct one by title match
	// - Returning only the pipeline MR, not other MRs
}

func TestBackend_CreatePipelineMR_ReturnsURLInPullRequest(t *testing.T) {
	t.Skip("Integration test requiring GitLab API access")
	// This test verifies that CreatePipelineMR returns a PullRequest with a URL
	// When we create an MR, the returned PullRequest should have a URL field
	// populated with the web URL of the merge request
	backend := New()
	repo := &models.Repository{
		Hostname: "gitlab.com",
		Path:     "test/project",
	}
	branches := []*models.Branch{
		{
			Name:       "main",
			Repository: repo,
		},
	}

	pr, err := backend.CreatePipelineMR(repo, branches)
	if err != nil {
		t.Fatalf("CreatePipelineMR failed: %v", err)
	}

	if pr.URL == "" {
		t.Error("Expected PullRequest.URL to be set, got empty string")
	}

	expectedURLPrefix := "https://gitlab.com/test/project/-/merge_requests/"
	if !strings.HasPrefix(pr.URL, expectedURLPrefix) {
		t.Errorf("Expected URL to start with %q, got: %s", expectedURLPrefix, pr.URL)
	}
}
