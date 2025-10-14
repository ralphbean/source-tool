// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package gitlab

import (
	"context"
	"strings"
	"testing"

	"github.com/slsa-framework/source-tool/pkg/slsa"
	"github.com/slsa-framework/source-tool/pkg/sourcetool/models"
)

func TestNew(t *testing.T) {
	backend := New()
	if backend == nil {
		t.Fatal("New() returned nil")
	}
	if backend.authenticator == nil {
		t.Error("New() backend.authenticator is nil")
	}
}

func TestGetGitLabConnection_NilRepository(t *testing.T) {
	backend := New()
	_, err := backend.getGitLabConnection(nil, "main")
	if err == nil {
		t.Fatal("Expected error when repository is nil, got nil")
	}
	if !strings.Contains(err.Error(), "repository is nil") {
		t.Errorf("Expected error message to contain 'repository is nil', got: %v", err)
	}
}

func TestGetGitLabConnection_EmptyPath(t *testing.T) {
	backend := New()
	repo := &models.Repository{
		Hostname: "gitlab.com",
		Path:     "",
	}
	_, err := backend.getGitLabConnection(repo, "main")
	if err == nil {
		t.Fatal("Expected error when repository path is empty, got nil")
	}
	if !strings.Contains(err.Error(), "repository path not set") {
		t.Errorf("Expected error message to contain 'repository path not set', got: %v", err)
	}
}

func TestControlImplementationMessage(t *testing.T) {
	backend := New()

	tests := []struct {
		name         string
		ctrlName     slsa.ControlName
		wantContains string
	}{
		{
			name:         "ProvenanceAvailable",
			ctrlName:     slsa.ProvenanceAvailable,
			wantContains: "provenance",
		},
		{
			name:         "TagHygiene",
			ctrlName:     slsa.TagHygiene,
			wantContains: "Tag protections",
		},
		{
			name:         "ReviewEnforced",
			ctrlName:     slsa.ReviewEnforced,
			wantContains: "Merge request approval",
		},
		{
			name:         "ContinuityEnforced",
			ctrlName:     slsa.ContinuityEnforced,
			wantContains: "Force push",
		},
		{
			name:         "PolicyAvailable",
			ctrlName:     slsa.PolicyAvailable,
			wantContains: "policy",
		},
		{
			name:         "Unknown control",
			ctrlName:     "UNKNOWN_CONTROL",
			wantContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := backend.controlImplementationMessage(tt.ctrlName)
			if tt.wantContains == "" {
				if msg != "" {
					t.Errorf("Expected empty message for unknown control, got: %s", msg)
				}
			} else {
				if !strings.Contains(strings.ToLower(msg), strings.ToLower(tt.wantContains)) {
					t.Errorf("Expected message to contain %q, got: %s", tt.wantContains, msg)
				}
			}
		})
	}
}

func TestGetTagControls_NotImplemented(t *testing.T) {
	backend := New()
	tag := &models.Tag{
		Name: "v1.0.0",
	}

	_, err := backend.GetTagControls(context.Background(), tag)
	if err == nil {
		t.Fatal("Expected error for not implemented function, got nil")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("Expected 'not yet implemented' error, got: %v", err)
	}
	// Should reference the upstream GitLab issue
	if !strings.Contains(err.Error(), "579382") {
		t.Errorf("Expected error to reference GitLab issue #579382, got: %v", err)
	}
}

func TestControlConfigurationDescr(t *testing.T) {
	backend := New()

	tests := []struct {
		name         string
		config       models.ControlConfiguration
		wantContains []string
	}{
		{
			name:   "CONFIG_BRANCH_RULES",
			config: models.CONFIG_BRANCH_RULES,
			wantContains: []string{
				"force push",
				"delete protection",
				"branch",
			},
		},
		{
			name:   "CONFIG_GEN_PROVENANCE",
			config: models.CONFIG_GEN_PROVENANCE,
			wantContains: []string{
				"merge request",
				"provenance",
			},
		},
		{
			name:   "CONFIG_POLICY",
			config: models.CONFIG_POLICY,
			wantContains: []string{
				"policy",
			},
		},
		{
			name:   "CONFIG_TAG_RULES",
			config: models.CONFIG_TAG_RULES,
			wantContains: []string{
				"tag",
			},
		},
		{
			name:         "Unknown config",
			config:       "UNKNOWN_CONFIG",
			wantContains: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch := &models.Branch{
				Name: "main",
				Repository: &models.Repository{
					Path: "test/project",
				},
			}

			descr := backend.ControlConfigurationDescr(branch, tt.config)

			if tt.wantContains == nil {
				if descr != "" {
					t.Errorf("Expected empty description for unknown config, got: %s", descr)
				}
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(strings.ToLower(descr), strings.ToLower(want)) {
					t.Errorf("Expected description to contain %q, got: %s", want, descr)
				}
			}

			// Check that the repo path is included
			if !strings.Contains(descr, "test/project") {
				t.Errorf("Expected description to contain repo path, got: %s", descr)
			}
		})
	}
}

func TestControlConfigurationDescr_NilRepository(t *testing.T) {
	backend := New()

	branch := &models.Branch{
		Name:       "main",
		Repository: nil,
	}

	descr := backend.ControlConfigurationDescr(branch, models.CONFIG_BRANCH_RULES)

	// Should use default "your repository" text
	if !strings.Contains(descr, "your repository") {
		t.Errorf("Expected description to contain 'your repository' when repo is nil, got: %s", descr)
	}
}

func TestGetRecommendedAction(t *testing.T) {
	backend := New()
	repo := &models.Repository{Path: "test/project"}
	branch := &models.Branch{Name: "main"}

	tests := []struct {
		name    string
		control slsa.ControlName
		state   slsa.ControlState
		wantNil bool
		wantMsg string
		wantCmd string
	}{
		{
			name:    "ProvenanceAvailable - InProgress",
			control: slsa.ProvenanceAvailable,
			state:   slsa.StateInProgress,
			wantMsg: "merge request",
		},
		{
			name:    "ProvenanceAvailable - NotEnabled",
			control: slsa.ProvenanceAvailable,
			state:   slsa.StateNotEnabled,
			wantMsg: "generating provenance",
			wantCmd: "setup controls",
		},
		{
			name:    "ProvenanceAvailable - Active",
			control: slsa.ProvenanceAvailable,
			state:   slsa.StateActive,
			wantNil: true,
		},
		{
			name:    "ContinuityEnforced - NotEnabled",
			control: slsa.ContinuityEnforced,
			state:   slsa.StateNotEnabled,
			wantMsg: "branch force push",
			wantCmd: "CONFIG_BRANCH_RULES",
		},
		{
			name:    "ContinuityEnforced - Active",
			control: slsa.ContinuityEnforced,
			state:   slsa.StateActive,
			wantNil: true,
		},
		{
			name:    "TagHygiene - NotEnabled",
			control: slsa.TagHygiene,
			state:   slsa.StateNotEnabled,
			wantMsg: "tag",
			wantCmd: "CONFIG_TAG_RULES",
		},
		{
			name:    "TagHygiene - Active",
			control: slsa.TagHygiene,
			state:   slsa.StateActive,
			wantNil: true,
		},
		{
			name:    "Unknown control",
			control: "UNKNOWN_CONTROL",
			state:   slsa.StateNotEnabled,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := backend.getRecommendedAction(repo, branch, tt.control, tt.state)

			if tt.wantNil {
				if action != nil {
					t.Errorf("Expected nil action, got: %+v", action)
				}
				return
			}

			if action == nil {
				t.Fatal("Expected non-nil action, got nil")
			}

			if tt.wantMsg != "" {
				if !strings.Contains(strings.ToLower(action.Message), strings.ToLower(tt.wantMsg)) {
					t.Errorf("Expected message to contain %q, got: %s", tt.wantMsg, action.Message)
				}
			}

			if tt.wantCmd != "" {
				if !strings.Contains(action.Command, tt.wantCmd) {
					t.Errorf("Expected command to contain %q, got: %s", tt.wantCmd, action.Command)
				}
			}
		})
	}
}

func TestGetBranchControlsAtCommit_NilCommit(t *testing.T) {
	backend := New()
	repo := &models.Repository{
		Hostname: "gitlab.com",
		Path:     "test/project",
	}
	branch := &models.Branch{
		Name:       "main",
		Repository: repo,
	}

	_, err := backend.GetBranchControlsAtCommit(context.Background(), repo, branch, nil)
	if err == nil {
		t.Fatal("Expected error when commit is nil, got nil")
	}
	if !strings.Contains(err.Error(), "commit is not set") {
		t.Errorf("Expected error message to contain 'commit is not set', got: %v", err)
	}
}

func TestConfigureControls_UnsupportedConfig(t *testing.T) {
	backend := New()
	repo := &models.Repository{
		Hostname: "gitlab.com",
		Path:     "test/project",
	}
	branch := &models.Branch{
		Name:       "main",
		Repository: repo,
	}

	err := backend.ConfigureControls(repo, []*models.Branch{branch}, []models.ControlConfiguration{"UNSUPPORTED_CONFIG"})
	if err == nil {
		t.Fatal("Expected error for unsupported configuration, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported configuration") {
		t.Errorf("Expected error message to contain 'unsupported configuration', got: %v", err)
	}
}

func TestControlPrecheck(t *testing.T) {
	backend := New()
	repo := &models.Repository{
		Hostname: "gitlab.com",
		Path:     "test/project",
	}
	branch := &models.Branch{
		Name:       "main",
		Repository: repo,
	}

	ok, msg, fn, err := backend.ControlPrecheck(repo, []*models.Branch{branch}, models.CONFIG_BRANCH_RULES)
	if err != nil {
		t.Errorf("ControlPrecheck() error = %v, want nil", err)
	}
	if !ok {
		t.Error("ControlPrecheck() ok = false, want true")
	}
	if msg != "" {
		t.Errorf("ControlPrecheck() msg = %q, want empty", msg)
	}
	if fn != nil {
		t.Error("ControlPrecheck() fn != nil, want nil")
	}
}

// Note: Testing GetBranchControls, GetBranchControlsAtCommit (with real GitLab API calls),
// GetLatestCommit, and ConfigureControls with actual API interactions would require:
// - Setting up a mock GitLab server using httptest
// - Mocking all the GitLab API endpoints
// - Or using integration tests with a real GitLab instance
//
// These are skipped in unit tests but should be covered in integration tests.

func TestConfigureControls_BranchProtectionAlreadyEnabled(t *testing.T) {
	t.Skip("This test would require mocking the GitLab API to return 'already exists' errors")
	// This would test that ConfigureControls handles models.ErrProtectionAlreadyInPlace
	// and logs appropriately without returning an error
}

func TestGetBranchControlsAtCommit_Integration(t *testing.T) {
	t.Skip("Integration test requiring GitLab API access")
	// This would test the full flow of GetBranchControlsAtCommit including:
	// - Getting GitLab connection
	// - Fetching branch controls
	// - Mapping controls to ControlSetStatus
	// - Populating recommended actions
}
