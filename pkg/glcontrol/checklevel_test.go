// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package glcontrol

import (
	"testing"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/slsa-framework/source-tool/pkg/provenance"
	"github.com/slsa-framework/source-tool/pkg/slsa"
)

func TestGetBranchFromRef(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{
			name:     "full ref",
			ref:      "refs/heads/main",
			expected: "main",
		},
		{
			name:     "full ref with slashes",
			ref:      "refs/heads/feature/my-feature",
			expected: "feature/my-feature",
		},
		{
			name:     "already branch name",
			ref:      "develop",
			expected: "develop",
		},
		{
			name:     "empty string",
			ref:      "",
			expected: "",
		},
		{
			name:     "tag ref",
			ref:      "refs/tags/v1.0.0",
			expected: "refs/tags/v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBranchFromRef(tt.ref)
			if got != tt.expected {
				t.Errorf("GetBranchFromRef(%q) = %q, want %q", tt.ref, got, tt.expected)
			}
		})
	}
}

func TestGLControlStatus_AddControl(t *testing.T) {
	baseTime := time.Now()
	pushTime := baseTime.Add(-1 * time.Hour) // Commit was pushed 1 hour ago

	tests := []struct {
		name           string
		commitPushTime time.Time
		controlSince   time.Time
		expectAdded    bool
		description    string
	}{
		{
			name:           "control enabled before commit",
			commitPushTime: pushTime,
			controlSince:   baseTime.Add(-2 * time.Hour), // Control enabled 2 hours ago
			expectAdded:    true,
			description:    "control existed when commit was pushed",
		},
		{
			name:           "control enabled after commit",
			commitPushTime: pushTime,
			controlSince:   baseTime, // Control enabled now (1 hour after push)
			expectAdded:    false,
			description:    "control did not exist when commit was pushed",
		},
		{
			name:           "control enabled at same time as commit",
			commitPushTime: pushTime,
			controlSince:   pushTime,
			expectAdded:    false,
			description:    "control enabled at exact same time (boundary case)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := &GLControlStatus{
				CommitPushTime: tt.commitPushTime,
				Controls:       slsa.Controls{},
			}

			control := &provenance.Control{
				Name:  "TEST_CONTROL",
				Since: timestamppb.New(tt.controlSince),
			}

			cs.AddControl(control)

			if tt.expectAdded {
				if len(cs.Controls) != 1 {
					t.Errorf("Expected 1 control to be added, got %d: %s", len(cs.Controls), tt.description)
				}
				if len(cs.Controls) > 0 && cs.Controls[0].GetName() != "TEST_CONTROL" {
					t.Errorf("Expected TEST_CONTROL, got %s", cs.Controls[0].GetName())
				}
			} else {
				if len(cs.Controls) != 0 {
					t.Errorf("Expected 0 controls to be added, got %d: %s", len(cs.Controls), tt.description)
				}
			}
		})
	}
}

func TestGLControlStatus_AddControl_Nil(t *testing.T) {
	cs := &GLControlStatus{
		CommitPushTime: time.Now(),
		Controls:       slsa.Controls{},
	}

	// Adding nil should not panic or add anything
	cs.AddControl(nil)

	if len(cs.Controls) != 0 {
		t.Errorf("Expected 0 controls after adding nil, got %d", len(cs.Controls))
	}
}

func TestGLControlStatus_AddControl_Multiple(t *testing.T) {
	baseTime := time.Now()
	pushTime := baseTime.Add(-1 * time.Hour)

	cs := &GLControlStatus{
		CommitPushTime: pushTime,
		Controls:       slsa.Controls{},
	}

	control1 := &provenance.Control{
		Name:  "CONTROL_1",
		Since: timestamppb.New(baseTime.Add(-2 * time.Hour)), // Before push
	}
	control2 := &provenance.Control{
		Name:  "CONTROL_2",
		Since: timestamppb.New(baseTime.Add(-3 * time.Hour)), // Before push
	}
	control3 := &provenance.Control{
		Name:  "CONTROL_3",
		Since: timestamppb.New(baseTime), // After push
	}

	// Add multiple controls at once
	cs.AddControl(control1, control2, control3, nil)

	// Should have added only control1 and control2 (control3 is after push, nil is ignored)
	if len(cs.Controls) != 2 {
		t.Errorf("Expected 2 controls, got %d", len(cs.Controls))
	}

	// Check that the right controls were added
	names := make(map[string]bool)
	for _, c := range cs.Controls {
		names[c.GetName()] = true
	}

	if !names["CONTROL_1"] || !names["CONTROL_2"] {
		t.Error("Expected CONTROL_1 and CONTROL_2 to be added")
	}
	if names["CONTROL_3"] {
		t.Error("Did not expect CONTROL_3 to be added (it was after push time)")
	}
}

// Note: The following functions make real GitLab API calls and would require
// mocking the GitLab API server with httptest or similar. For comprehensive
// testing, you would need to:
//
// 1. Create a mock GitLab server using httptest.NewServer
// 2. Configure the GitLab client to use the mock server's URL
// 3. Set up handlers for the various API endpoints
// 4. Test the control computation functions
//
// Example structure (not implemented here due to complexity):
//
// func TestGetBranchControls(t *testing.T) {
//     server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//         // Mock responses for:
//         // - GET /api/v4/projects/:id/protected_branches/:name
//         // - GET /api/v4/projects/:id
//         // - GET /api/v4/projects/:id/approval_rules
//         // - GET /api/v4/projects/:id/protected_tags
//     }))
//     defer server.Close()
//
//     client, _ := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL))
//     conn := NewGitLabConnectionWithClient("test/project", "refs/heads/main", client)
//
//     controls, err := conn.GetBranchControls(context.Background(), "refs/heads/main")
//     // ... assertions
// }

func TestComputeContinuityControl_NilProtectedBranch(t *testing.T) {
	client, err := gitlab.NewClient("test-token")
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	conn := NewGitLabConnectionWithClient("test/project", "main", client)

	// Passing nil should return nil (no control)
	control, err := conn.computeContinuityControl(nil, nil)
	if err != nil {
		t.Errorf("computeContinuityControl(nil) error = %v, want nil", err)
	}
	if control != nil {
		t.Errorf("computeContinuityControl(nil) = %v, want nil", control)
	}
}

func TestComputeReviewControl_NilProtectedBranch(t *testing.T) {
	client, err := gitlab.NewClient("test-token")
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	conn := NewGitLabConnectionWithClient("test/project", "main", client)

	// Passing nil should return nil (no control)
	control, err := conn.computeReviewControl(nil, nil)
	if err != nil {
		t.Errorf("computeReviewControl(nil) error = %v, want nil", err)
	}
	if control != nil {
		t.Errorf("computeReviewControl(nil) = %v, want nil", control)
	}
}
