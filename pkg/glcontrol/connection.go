// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package glcontrol

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

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

	opts := defaultOptions
	retryMax := int(opts.ApiRetries)

	var client *gitlab.Client
	var err error

	if hostname != "" && hostname != "gitlab.com" {
		// Use custom GitLab instance
		baseURL := fmt.Sprintf("https://%s/api/v4", hostname)
		client, err = gitlab.NewClient(token,
			gitlab.WithBaseURL(baseURL),
			gitlab.WithCustomRetryMax(retryMax))
	} else {
		// Use gitlab.com (default)
		client, err = gitlab.NewClient(token,
			gitlab.WithCustomRetryMax(retryMax))
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
		retryMax := int(glc.Options.ApiRetries)
		client, err := gitlab.NewClient(token,
			gitlab.WithCustomRetryMax(retryMax))
		if err != nil {
			return nil, fmt.Errorf("creating GitLab client with token: %w", err)
		}
		glc.client = client
		glc.Options.accessToken = token
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

// supportsOIDC checks if a GitLab version supports OIDC id_tokens
// OIDC support was added in GitLab 15.7
func supportsOIDC(version string) bool {
	// Parse version string (e.g., "15.7.0" -> 15.7)
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}

	// OIDC added in 15.7
	if major > 15 {
		return true
	}
	if major == 15 && minor >= 7 {
		return true
	}

	return false
}

// GetVersion retrieves the GitLab instance version
func (glc *GitLabConnection) GetVersion(ctx context.Context) (string, error) {
	version, _, err := glc.Client().Version.GetVersion()
	if err != nil {
		return "", fmt.Errorf("getting GitLab version: %w", err)
	}

	return version.Version, nil
}

// SupportsOIDC checks if the GitLab instance supports OIDC
func (glc *GitLabConnection) SupportsOIDC(ctx context.Context) (bool, error) {
	version, err := glc.GetVersion(ctx)
	if err != nil {
		return false, err
	}

	return supportsOIDC(version), nil
}
