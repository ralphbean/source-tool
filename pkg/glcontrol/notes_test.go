// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package glcontrol

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestGetNotesForCommit_InvalidCommitSHA(t *testing.T) {
	client, err := gitlab.NewClient("test-token")
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	conn := NewGitLabConnectionWithClient("test/project", "main", client)

	tests := []struct {
		name   string
		commit string
	}{
		{"empty commit", ""},
		{"too short", "abc123"},
		{"too long", "e573149ab3e574abc2e5a151a04acfaf2a59b453extra"},
		{"39 chars", "e573149ab3e574abc2e5a151a04acfaf2a59b45"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := conn.GetNotesForCommit(context.Background(), tt.commit)
			if err == nil {
				t.Errorf("Expected error for invalid commit %q, got nil", tt.commit)
			}
			if err.Error() != "invalid commit string" {
				t.Errorf("Expected 'invalid commit string' error, got: %v", err)
			}
		})
	}
}

func TestGetNotesForCommit_ShardedPath(t *testing.T) {
	commit := "e573149ab3e574abc2e5a151a04acfaf2a59b453"
	noteContent := "test note content"
	encodedContent := base64.StdEncoding.EncodeToString([]byte(noteContent))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expect sharded path: e5/73149ab3e574abc2e5a151a04acfaf2a59b453
		expectedPath := "/api/v4/projects/test/project/repository/files/e5/73149ab3e574abc2e5a151a04acfaf2a59b453"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Check ref parameter
		ref := r.URL.Query().Get("ref")
		if ref != "refs/notes/commits" {
			t.Errorf("Expected ref 'refs/notes/commits', got %s", ref)
		}

		file := &gitlab.File{
			FileName: "e573149ab3e574abc2e5a151a04acfaf2a59b453",
			Content:  encodedContent,
			Encoding: "base64",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(file) //nolint:errcheck
	}))
	defer server.Close()

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL+"/api/v4"))
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	conn := NewGitLabConnectionWithClient("test/project", "main", client)

	notes, err := conn.GetNotesForCommit(context.Background(), commit)
	if err != nil {
		t.Fatalf("GetNotesForCommit() error = %v, want nil", err)
	}
	if notes != noteContent {
		t.Errorf("GetNotesForCommit() = %q, want %q", notes, noteContent)
	}
}

func TestGetNotesForCommit_TopLevelPath(t *testing.T) {
	commit := "e573149ab3e574abc2e5a151a04acfaf2a59b453"
	noteContent := "test note content"
	encodedContent := base64.StdEncoding.EncodeToString([]byte(noteContent))

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if callCount == 1 {
			// First call with sharded path - return 404
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Second call with top-level path
		expectedPath := "/api/v4/projects/test/project/repository/files/e573149ab3e574abc2e5a151a04acfaf2a59b453"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		file := &gitlab.File{
			FileName: commit,
			Content:  encodedContent,
			Encoding: "base64",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(file) //nolint:errcheck
	}))
	defer server.Close()

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL+"/api/v4"))
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	conn := NewGitLabConnectionWithClient("test/project", "main", client)

	notes, err := conn.GetNotesForCommit(context.Background(), commit)
	if err != nil {
		t.Fatalf("GetNotesForCommit() error = %v, want nil", err)
	}
	if notes != noteContent {
		t.Errorf("GetNotesForCommit() = %q, want %q", notes, noteContent)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 API calls (sharded + top-level), got %d", callCount)
	}
}

func TestGetNotesForCommit_NotFound(t *testing.T) {
	commit := "e573149ab3e574abc2e5a151a04acfaf2a59b453"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return 404
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL+"/api/v4"))
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	conn := NewGitLabConnectionWithClient("test/project", "main", client)

	notes, err := conn.GetNotesForCommit(context.Background(), commit)
	if err != nil {
		t.Fatalf("GetNotesForCommit() error = %v, want nil (404 should return empty string)", err)
	}
	if notes != "" {
		t.Errorf("GetNotesForCommit() = %q, want empty string for 404", notes)
	}
}

func TestGetNotesForCommit_PlainTextEncoding(t *testing.T) {
	commit := "e573149ab3e574abc2e5a151a04acfaf2a59b453"
	noteContent := "plain text note"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file := &gitlab.File{
			FileName: commit,
			Content:  noteContent,
			Encoding: "text", // Not base64
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(file) //nolint:errcheck
	}))
	defer server.Close()

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL+"/api/v4"))
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	conn := NewGitLabConnectionWithClient("test/project", "main", client)

	notes, err := conn.GetNotesForCommit(context.Background(), commit)
	if err != nil {
		t.Fatalf("GetNotesForCommit() error = %v, want nil", err)
	}
	if notes != noteContent {
		t.Errorf("GetNotesForCommit() = %q, want %q", notes, noteContent)
	}
}

func TestGetNotesForCommit_InvalidBase64(t *testing.T) {
	commit := "e573149ab3e574abc2e5a151a04acfaf2a59b453"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file := &gitlab.File{
			FileName: commit,
			Content:  "!!!invalid-base64!!!",
			Encoding: "base64",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(file) //nolint:errcheck
	}))
	defer server.Close()

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL+"/api/v4"))
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	conn := NewGitLabConnectionWithClient("test/project", "main", client)

	_, err = conn.GetNotesForCommit(context.Background(), commit)
	if err == nil {
		t.Fatal("Expected error for invalid base64 content, got nil")
	}
	if err.Error() != "failed to decode base64 content: illegal base64 data at input byte 0" {
		t.Errorf("Unexpected error message: %v", err)
	}
}
