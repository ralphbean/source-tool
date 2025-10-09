// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package glcontrol

import (
	"os"
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestNewGitLabConnection_NoToken(t *testing.T) {
	// Save and clear the token
	oldToken := os.Getenv(tokenEnvVar)
	os.Unsetenv(tokenEnvVar)
	defer func() {
		if oldToken != "" {
			os.Setenv(tokenEnvVar, oldToken)
		}
	}()

	_, err := NewGitLabConnection("test/project", "main")
	if err == nil {
		t.Fatal("Expected error when GITLAB_TOKEN is not set, got nil")
	}
	if err.Error() != "GITLAB_TOKEN environment variable is not set" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestNewGitLabConnection_WithToken(t *testing.T) {
	// Set a test token
	oldToken := os.Getenv(tokenEnvVar)
	os.Setenv(tokenEnvVar, "test-token")
	defer func() {
		if oldToken != "" {
			os.Setenv(tokenEnvVar, oldToken)
		} else {
			os.Unsetenv(tokenEnvVar)
		}
	}()

	conn, err := NewGitLabConnection("test/project", "main")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if conn == nil {
		t.Fatal("Expected connection, got nil")
	}
	if conn.ProjectID() != "test/project" {
		t.Errorf("Expected project ID 'test/project', got: %v", conn.ProjectID())
	}
	if conn.GetFullRef() != "main" {
		t.Errorf("Expected ref 'main', got: %v", conn.GetFullRef())
	}
}

func TestNewGitLabConnectionWithHostname_GitLabCom(t *testing.T) {
	oldToken := os.Getenv(tokenEnvVar)
	os.Setenv(tokenEnvVar, "test-token")
	defer func() {
		if oldToken != "" {
			os.Setenv(tokenEnvVar, oldToken)
		} else {
			os.Unsetenv(tokenEnvVar)
		}
	}()

	tests := []struct {
		name     string
		hostname string
	}{
		{"empty hostname defaults to gitlab.com", ""},
		{"explicit gitlab.com", "gitlab.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewGitLabConnectionWithHostname("test/project", "main", tt.hostname)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if conn == nil {
				t.Fatal("Expected connection, got nil")
			}
			// Default gitlab.com client should be created
			if conn.Client() == nil {
				t.Fatal("Expected client to be set")
			}
		})
	}
}

func TestNewGitLabConnectionWithHostname_CustomInstance(t *testing.T) {
	oldToken := os.Getenv(tokenEnvVar)
	os.Setenv(tokenEnvVar, "test-token")
	defer func() {
		if oldToken != "" {
			os.Setenv(tokenEnvVar, oldToken)
		} else {
			os.Unsetenv(tokenEnvVar)
		}
	}()

	conn, err := NewGitLabConnectionWithHostname("test/project", "main", "gitlab.example.com")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if conn == nil {
		t.Fatal("Expected connection, got nil")
	}
	if conn.Client() == nil {
		t.Fatal("Expected client to be set")
	}
	// Verify it's a custom instance (client should be configured with custom base URL)
	// The client's base URL should be https://gitlab.example.com/api/v4
	if conn.Client().BaseURL() == nil {
		t.Fatal("Expected base URL to be set")
	}
	expectedURL := "https://gitlab.example.com/api/v4/"
	if conn.Client().BaseURL().String() != expectedURL {
		t.Errorf("Expected base URL '%s', got: %s", expectedURL, conn.Client().BaseURL().String())
	}
}

func TestNewGitLabConnectionWithClient(t *testing.T) {
	client, err := gitlab.NewClient("test-token")
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	conn := NewGitLabConnectionWithClient("test/project", "refs/heads/main", client)
	if conn == nil {
		t.Fatal("Expected connection, got nil")
	}
	if conn.Client() != client {
		t.Error("Expected same client instance")
	}
	if conn.ProjectID() != "test/project" {
		t.Errorf("Expected project ID 'test/project', got: %v", conn.ProjectID())
	}
	if conn.GetFullRef() != "refs/heads/main" {
		t.Errorf("Expected ref 'refs/heads/main', got: %v", conn.GetFullRef())
	}
}

func TestGitLabConnection_Getters(t *testing.T) {
	client, err := gitlab.NewClient("test-token")
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	projectID := 12345
	ref := "refs/heads/develop"
	conn := NewGitLabConnectionWithClient(projectID, ref, client)

	if conn.ProjectID() != projectID {
		t.Errorf("ProjectID() = %v, want %v", conn.ProjectID(), projectID)
	}
	if conn.GetFullRef() != ref {
		t.Errorf("GetFullRef() = %v, want %v", conn.GetFullRef(), ref)
	}
	if conn.Client() != client {
		t.Error("Client() returned different client")
	}
}

func TestGitLabConnection_WithAuthToken(t *testing.T) {
	client, err := gitlab.NewClient("old-token")
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	conn := NewGitLabConnectionWithClient("test/project", "main", client)

	// Test setting a new token
	newConn, err := conn.WithAuthToken("new-token")
	if err != nil {
		t.Fatalf("WithAuthToken() error = %v, want nil", err)
	}
	if newConn != conn {
		t.Error("WithAuthToken() should return same connection instance")
	}
	// Client should have been updated
	if newConn.Client() == client {
		t.Error("Expected client to be updated with new token")
	}
}

func TestGitLabConnection_WithAuthToken_EmptyString(t *testing.T) {
	client, err := gitlab.NewClient("test-token")
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	conn := NewGitLabConnectionWithClient("test/project", "main", client)

	// Test with empty token (should be no-op)
	newConn, err := conn.WithAuthToken("")
	if err != nil {
		t.Fatalf("WithAuthToken() error = %v, want nil", err)
	}
	if newConn != conn {
		t.Error("WithAuthToken() should return same connection instance")
	}
	// Client should remain unchanged
	if newConn.Client() != client {
		t.Error("Expected client to remain unchanged when token is empty")
	}
}

// Note: GetRepoUri, GetPriorCommit, GetLatestCommit, and GetDefaultBranch
// make real API calls and would require mocking the GitLab API server.
// These are tested in integration tests or would need httptest server setup.
// For now, we test the connection setup and basic functionality.

func TestGitLabConnection_IntegrationSkipped(t *testing.T) {
	t.Skip("Integration tests requiring GitLab API are skipped in unit tests")

	// Example of what integration tests would look like:
	// ctx := context.Background()
	// conn := setupTestConnection(t)
	//
	// uri, err := conn.GetRepoUri(ctx)
	// if err != nil {
	//     t.Fatalf("GetRepoUri() error = %v", err)
	// }
	// if uri == "" {
	//     t.Error("Expected non-empty URI")
	// }
}
