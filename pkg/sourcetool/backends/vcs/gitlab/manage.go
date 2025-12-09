// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package gitlab

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/slsa-framework/source-tool/pkg/sourcetool/models"
)

const defaultGitLabHostname = "gitlab.com"

// CreatePipelineMR creates a merge request to add SLSA source CI pipeline
func (b *Backend) CreatePipelineMR(r *models.Repository, branches []*models.Branch) (*models.PullRequest, error) {
	if len(branches) == 0 {
		return nil, errors.New("no branches specified")
	}

	ctx := context.Background()

	// Get GitLab connection
	glc, err := b.getGitLabConnection(r, "")
	if err != nil {
		return nil, fmt.Errorf("getting GitLab connection: %w", err)
	}

	// Detect OIDC support
	supportsOIDC, err := glc.SupportsOIDC(ctx)
	if err != nil {
		return nil, fmt.Errorf("checking OIDC support: %w", err)
	}

	// Choose appropriate template
	var pipelineTemplate string
	if supportsOIDC {
		pipelineTemplate = getOIDCPipelineTemplate()
	} else {
		pipelineTemplate = getCosignPipelineTemplate()
		// TODO: For cosign, we need to prompt user to set up COSIGN_PRIVATE_KEY
		// For now, we'll create the template and document the requirement
	}

	// Get the project to find default branch
	project, _, err := glc.Client().Projects.GetProject(glc.ProjectID(), nil)
	if err != nil {
		return nil, fmt.Errorf("getting project info: %w", err)
	}
	defaultBranch := project.DefaultBranch

	// Create a feature branch name
	featureBranch := "slsa-source-provenance"

	// Create the branch from default branch
	_, _, err = glc.Client().Branches.CreateBranch(glc.ProjectID(), &gitlab.CreateBranchOptions{
		Branch: gitlab.Ptr(featureBranch),
		Ref:    gitlab.Ptr(defaultBranch),
	})
	if err != nil {
		// Branch might already exist, which is fine
		if !strings.Contains(err.Error(), "already exists") {
			return nil, fmt.Errorf("creating branch: %w", err)
		}
	}

	// Prepare file actions for commit
	actions := []*gitlab.CommitActionOptions{
		// Create .gitlab/slsa-source.yml
		{
			Action:   gitlab.Ptr(gitlab.FileCreate),
			FilePath: gitlab.Ptr(pipelinePathOIDC),
			Content:  gitlab.Ptr(pipelineTemplate),
		},
	}

	// Read existing .gitlab-ci.yml if it exists
	existingCI, _, err := glc.Client().RepositoryFiles.GetFile(
		glc.ProjectID(),
		gitlabCIPath,
		&gitlab.GetFileOptions{Ref: gitlab.Ptr(featureBranch)},
	)

	var ciContent string
	if err != nil {
		// File doesn't exist, create new
		ciContent = appendToGitLabCI("", getGitLabCIInclude())
		actions = append(actions, &gitlab.CommitActionOptions{
			Action:   gitlab.Ptr(gitlab.FileCreate),
			FilePath: gitlab.Ptr(gitlabCIPath),
			Content:  gitlab.Ptr(ciContent),
		})
	} else {
		// File exists, update it
		var decodedContent string
		if existingCI.Encoding == "base64" {
			decoded, err := base64.StdEncoding.DecodeString(existingCI.Content)
			if err != nil {
				return nil, fmt.Errorf("decoding base64 content: %w", err)
			}
			decodedContent = string(decoded)
		} else {
			decodedContent = existingCI.Content
		}

		ciContent = appendToGitLabCI(decodedContent, getGitLabCIInclude())
		actions = append(actions, &gitlab.CommitActionOptions{
			Action:   gitlab.Ptr(gitlab.FileUpdate),
			FilePath: gitlab.Ptr(gitlabCIPath),
			Content:  gitlab.Ptr(ciContent),
		})
	}

	// Commit the changes
	_, _, err = glc.Client().Commits.CreateCommit(glc.ProjectID(), &gitlab.CreateCommitOptions{
		Branch:        gitlab.Ptr(featureBranch),
		CommitMessage: gitlab.Ptr(pipelineCommitMessage),
		Actions:       actions,
	})
	if err != nil {
		return nil, fmt.Errorf("creating commit: %w", err)
	}

	// Create merge request
	mr, _, err := glc.Client().MergeRequests.CreateMergeRequest(glc.ProjectID(), &gitlab.CreateMergeRequestOptions{
		Title:        gitlab.Ptr(pipelineCommitMessage),
		Description:  gitlab.Ptr(pipelinePRBody),
		SourceBranch: gitlab.Ptr(featureBranch),
		TargetBranch: gitlab.Ptr(defaultBranch),
	})
	if err != nil {
		return nil, fmt.Errorf("creating merge request: %w", err)
	}

	// Construct the MR URL
	hostname := r.Hostname
	if hostname == "" {
		hostname = defaultGitLabHostname
	}
	mrURL := fmt.Sprintf("https://%s/%s/-/merge_requests/%d", hostname, r.Path, mr.IID)

	return &models.PullRequest{
		Title:  mr.Title,
		Body:   mr.Description,
		Number: mr.IID,
		Repo:   r,
		URL:    mrURL,
	}, nil
}

// FindPipelineMR searches for an existing pipeline MR
func (b *Backend) FindPipelineMR(ctx context.Context, r *models.Repository) (*models.PullRequest, error) {
	glc, err := b.getGitLabConnection(r, "")
	if err != nil {
		return nil, fmt.Errorf("getting GitLab connection: %w", err)
	}

	// List open merge requests
	opts := &gitlab.ListProjectMergeRequestsOptions{
		State: gitlab.Ptr("opened"),
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}

	mrs, _, err := glc.Client().MergeRequests.ListProjectMergeRequests(glc.ProjectID(), opts)
	if err != nil {
		return nil, fmt.Errorf("listing merge requests: %w", err)
	}

	// Search for MR with pipeline commit message in title
	for _, mr := range mrs {
		if strings.Contains(mr.Title, pipelineCommitMessage) {
			// Construct the MR URL
			hostname := r.Hostname
			if hostname == "" {
				hostname = defaultGitLabHostname
			}
			mrURL := fmt.Sprintf("https://%s/%s/-/merge_requests/%d", hostname, r.Path, mr.IID)

			return &models.PullRequest{
				Title:  mr.Title,
				Body:   mr.Description,
				Number: mr.IID,
				Repo:   r,
				URL:    mrURL,
			}, nil
		}
	}

	return nil, nil
}
