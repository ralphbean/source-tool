// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package glcontrol

import (
	"context"
	"fmt"
	"os"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

const tokenEnvVar = "GITLAB_TOKEN" //nolint:gosec // These are not credentials

// Manages a connection to a GitLab repository.
type GitLabConnection struct {
	client           *gitlab.Client
	Options          Options
	projectID        interface{} // Can be string (path with namespace) or int (project ID)
	ref              string
}

func NewGitLabConnection(projectID interface{}, ref string) (*GitLabConnection, error) {
	return NewGitLabConnectionWithHostname(projectID, ref, "")
}

func NewGitLabConnectionWithHostname(projectID interface{}, ref, hostname string) (*GitLabConnection, error) {
	token := os.Getenv(tokenEnvVar)
	if token == "" {
		return nil, fmt.Errorf("GITLAB_TOKEN environment variable is not set")
	}

	var client *gitlab.Client
	var err error

	if hostname != "" && hostname != "gitlab.com" {
		// Use custom GitLab instance
		baseURL := fmt.Sprintf("https://%s/api/v4", hostname)
		client, err = gitlab.NewClient(token, gitlab.WithBaseURL(baseURL))
	} else {
		// Use gitlab.com (default)
		client, err = gitlab.NewClient(token)
	}

	if err != nil {
		return nil, fmt.Errorf("creating GitLab client: %w", err)
	}

	return NewGitLabConnectionWithClient(projectID, ref, client), nil
}

func NewGitLabConnectionWithClient(projectID interface{}, ref string, client *gitlab.Client) *GitLabConnection {
	opts := defaultOptions

	return &GitLabConnection{
		client:    client,
		projectID: projectID,
		ref:       ref,
		Options:   opts,
	}
}

func (glc *GitLabConnection) Client() *gitlab.Client {
	return glc.client
}

func (glc *GitLabConnection) ProjectID() interface{} {
	return glc.projectID
}

func (glc *GitLabConnection) GetFullRef() string {
	return glc.ref
}

// WithAuthToken sets the authentication token for the client.
// If the token is the empty string this is a no-op.
func (glc *GitLabConnection) WithAuthToken(token string) (*GitLabConnection, error) {
	if token != "" {
		client, err := gitlab.NewClient(token)
		if err != nil {
			return nil, fmt.Errorf("creating GitLab client with token: %w", err)
		}
		glc.client = client
	}
	return glc, nil
}

// Returns the URI of the repo this connection tracks.
func (glc *GitLabConnection) GetRepoUri(ctx context.Context) (string, error) {
	project, _, err := glc.Client().Projects.GetProject(glc.projectID, nil)
	if err != nil {
		return "", fmt.Errorf("getting project info: %w", err)
	}
	return project.WebURL, nil
}

// Gets the previous commit to 'sha' if it has one.
// If there are more than one parents this fails with an error.
func (glc *GitLabConnection) GetPriorCommit(ctx context.Context, sha string) (string, error) {
	commit, _, err := glc.Client().Commits.GetCommit(glc.projectID, sha, nil)
	if err != nil {
		return "", fmt.Errorf("cannot get commit data for %s: %w", sha, err)
	}

	if len(commit.ParentIDs) == 0 {
		return "", fmt.Errorf("there is no commit earlier than %s, that isn't yet supported", sha)
	}

	if len(commit.ParentIDs) > 1 && !glc.Options.AllowMergeCommits {
		return "", fmt.Errorf("commit %s has more than one parent (%v), which is not supported", sha, commit.ParentIDs)
	}

	return commit.ParentIDs[0], nil
}

func (glc *GitLabConnection) GetLatestCommit(ctx context.Context, targetBranch string) (string, error) {
	branch, _, err := glc.Client().Branches.GetBranch(glc.projectID, targetBranch, nil)
	if err != nil {
		return "", fmt.Errorf("could not get info on specified branch %s: %w", targetBranch, err)
	}
	return branch.Commit.ID, nil
}

// GetDefaultBranch reads the default repository branch from the GitLab API
func (glc *GitLabConnection) GetDefaultBranch(ctx context.Context) (string, error) {
	project, _, err := glc.Client().Projects.GetProject(glc.projectID, nil)
	if err != nil {
		return "", fmt.Errorf("fetching project data: %w", err)
	}

	return project.DefaultBranch, nil
}
