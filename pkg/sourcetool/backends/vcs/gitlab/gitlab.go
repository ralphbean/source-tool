// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package gitlab

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/slsa-framework/source-tool/pkg/auth"
	"github.com/slsa-framework/source-tool/pkg/glcontrol"
	"github.com/slsa-framework/source-tool/pkg/provenance"
	"github.com/slsa-framework/source-tool/pkg/slsa"
	"github.com/slsa-framework/source-tool/pkg/sourcetool/models"
)

func New() *Backend {
	return &Backend{
		authenticator: auth.New(),
	}
}

// Backend implements the GitLab sourcetool backend
type Backend struct {
	authenticator *auth.Authenticator
}

// getGitLabConnection builds a GitLab connector to a repository
func (b *Backend) getGitLabConnection(repository *models.Repository, ref string) (*glcontrol.GitLabConnection, error) {
	if repository == nil {
		return nil, fmt.Errorf("unable to build GitLab connection, repository is nil")
	}

	if repository.Path == "" {
		return nil, errors.New("repository path not set")
	}

	// For GitLab, the project ID is the path with namespace (e.g., "group/project")
	projectID := repository.Path

	// Use the hostname from the repository for custom GitLab instances
	hostname := repository.Hostname
	if hostname == "" {
		hostname = "gitlab.com"
	}

	return glcontrol.NewGitLabConnectionWithHostname(projectID, ref, hostname)
}

func (b *Backend) GetBranchControls(ctx context.Context, r *models.Repository, branch *models.Branch) (*slsa.ControlSetStatus, error) {
	// Get latest commit
	glc, err := b.getGitLabConnection(branch.Repository, branch.FullRef())
	if err != nil {
		return nil, fmt.Errorf("getting GitLab connection: %w", err)
	}

	// Get the latest commit from the branch
	commit, err := glc.GetLatestCommit(ctx, glcontrol.GetBranchFromRef(branch.FullRef()))
	if err != nil {
		return nil, fmt.Errorf("fetching latest commit from %q: %w", branch.FullRef(), err)
	}

	return b.GetBranchControlsAtCommit(ctx, r, branch, &models.Commit{SHA: commit})
}

// GetBranchControlsAtCommit returns the controls for a branch at a specific commit
func (b *Backend) GetBranchControlsAtCommit(ctx context.Context, r *models.Repository, branch *models.Branch, commit *models.Commit) (*slsa.ControlSetStatus, error) {
	if commit == nil {
		return nil, errors.New("commit is not set")
	}
	glc, err := b.getGitLabConnection(branch.Repository, branch.FullRef())
	if err != nil {
		return nil, fmt.Errorf("getting GitLab connection: %w", err)
	}

	// Get the active controls
	activeControls, err := glc.GetBranchControls(ctx, branch.FullRef())
	if err != nil {
		return nil, fmt.Errorf("checking status: %w", err)
	}

	// Check for PROVENANCE_AVAILABLE by looking for git notes on the commit
	// GitLab stores attestations in git notes just like GitHub
	notes, err := glc.GetNotesForCommit(ctx, commit.SHA)
	if err != nil {
		return nil, fmt.Errorf("attempting to read provenance from commit %q: %w", commit.SHA, err)
	}
	if notes != "" {
		// Provenance attestation found
		activeControls.AddControl(&provenance.Control{
			Name: slsa.ProvenanceAvailable.String(),
		})
	} else {
		log.Printf("No provenance attestation found on %s", commit.SHA)
	}

	status := slsa.NewControlSetStatus()
	for i, ctrl := range status.Controls {
		if c := activeControls.GetControl(ctrl.Name); c != nil {
			t := c.GetSince().AsTime()
			status.Controls[i].Since = &t
			status.Controls[i].State = slsa.StateActive
			status.Controls[i].Message = b.controlImplementationMessage(slsa.ControlName(c.GetName()))
		}
	}

	// Populate the recommended actions
	for i := range status.Controls {
		action := b.getRecommendedAction(r, branch, status.Controls[i].Name, status.Controls[i].State)
		status.Controls[i].RecommendedAction = action
	}

	return status, nil
}

// controlImplementationMessage returns an implementation message to populate the
// status message when controls are active.
func (b *Backend) controlImplementationMessage(ctrlName slsa.ControlName) string {
	switch ctrlName {
	case slsa.ProvenanceAvailable:
		return "Signed provenance metadata is being published on every commit"
	case slsa.TagHygiene:
		return "Tag protections are configured in the repository"
	case slsa.ReviewEnforced:
		return "Merge request approval is enforced in the repository"
	case slsa.ContinuityEnforced:
		return "Force push and delete protection is enabled on the branch"
	case slsa.PolicyAvailable:
		return "The repository has published a policy"
	default:
		return ""
	}
}

// GetTagControls is not implemented for GitLab due to API limitations.
//
// GitLab does not currently provide the necessary API features to verify SLSA Source
// Level 2+ tag immutability requirements. Specifically:
//
// 1. GitLab has no mechanism to prevent tags from being force-pushed/updated to point
//    to different commits (no deny_update_tag or deny_force_push_tag push rule).
//
// 2. The Protected Tags API lacks timestamps (created_at, updated_at) needed to verify
//    when tag protections were enabled ("time in force" requirement).
//
// 3. No audit events exist for protected tag changes (tracked in GitLab #268122).
//
// While GitLab's deny_delete_tag push rule can prevent tag deletion, SLSA requires
// both deletion AND update prevention for tag immutability, which GitLab cannot
// currently enforce or verify.
//
// See https://gitlab.com/gitlab-org/gitlab/-/issues/579382 for the upstream feature
// request to add these capabilities.
func (b *Backend) GetTagControls(ctx context.Context, tag *models.Tag) (*slsa.Controls, error) {
	return nil, fmt.Errorf("not yet implemented: GitLab API lacks tag immutability verification capabilities (see https://gitlab.com/gitlab-org/gitlab/-/issues/579382)") // https://gitlab.com/gitlab-org/gitlab/-/issues/579382
}

func (b *Backend) ControlConfigurationDescr(branch *models.Branch, config models.ControlConfiguration) string {
	repo := branch.Repository
	if repo == nil {
		repo = &models.Repository{
			Path: "your repository",
		}
	}

	switch config {
	case models.CONFIG_BRANCH_RULES:
		return fmt.Sprintf(
			"Enable force push and delete protection on %s for branch %s",
			repo.Path, branch.Name,
		)
	case models.CONFIG_GEN_PROVENANCE:
		return fmt.Sprintf(
			"Open a merge request on %s to add the provenance generation pipeline",
			repo.Path,
		)
	case models.CONFIG_POLICY:
		return fmt.Sprintf(
			"Open a merge request on the SLSA policy repo to check-in %s SLSA source policy",
			repo.Path,
		)
	case models.CONFIG_TAG_RULES:
		return fmt.Sprintf(
			"Enable force push/update/delete protection for all tags in %s",
			repo.Path,
		)
	default:
		return ""
	}
}

// GetLatestCommit returns the latest commit from a branch
func (b *Backend) GetLatestCommit(ctx context.Context, r *models.Repository, branch *models.Branch) (*models.Commit, error) {
	gcx, err := b.getGitLabConnection(r, branch.FullRef())
	if err != nil {
		return nil, fmt.Errorf("building GitLab connector: %w", err)
	}

	sha, err := gcx.GetLatestCommit(ctx, glcontrol.GetBranchFromRef(branch.FullRef()))
	if err != nil {
		return nil, fmt.Errorf("reading latest commit: %w", err)
	}

	return &models.Commit{SHA: sha}, nil
}

// getRecommendedAction returns the recommended action based on the
// status of a SLSA control
func (b *Backend) getRecommendedAction(r *models.Repository, _ *models.Branch, control slsa.ControlName, state slsa.ControlState) *slsa.ControlRecommendedAction {
	//nolint:exhaustive // Not all drivers handle all controls
	switch control {
	case slsa.ProvenanceAvailable:
		switch state {
		case slsa.StateInProgress:
			return &slsa.ControlRecommendedAction{
				Message: "Wait for provenance generator merge request to merge",
			}
		case slsa.StateNotEnabled:
			return &slsa.ControlRecommendedAction{
				Message: "Start generating provenance",
				Command: fmt.Sprintf("sourcetool setup controls --config=%s %s", models.CONFIG_GEN_PROVENANCE, r.Path),
			}
		default:
			return nil
		}
	case slsa.ContinuityEnforced:
		if state == slsa.StateNotEnabled {
			return &slsa.ControlRecommendedAction{
				Message: "Enable branch force push/delete protection",
				Command: fmt.Sprintf("sourcetool setup controls --config=%s %s", models.CONFIG_BRANCH_RULES, r.Path),
			}
		}
		return nil
	case slsa.TagHygiene:
		if state == slsa.StateNotEnabled {
			return &slsa.ControlRecommendedAction{
				Message: "Enable tag push/update/delete protection",
				Command: fmt.Sprintf("sourcetool setup controls --config=%s %s", models.CONFIG_TAG_RULES, r.Path),
			}
		}
		return nil
	default:
		return nil
	}
}

// ConfigureControls configures the specified controls for the repository
func (b *Backend) ConfigureControls(r *models.Repository, branches []*models.Branch, configs []models.ControlConfiguration) error {
	ctx := context.Background()

	for _, config := range configs {
		switch config {
		case models.CONFIG_BRANCH_RULES:
			for _, branch := range branches {
				glc, err := b.getGitLabConnection(branch.Repository, branch.FullRef())
				if err != nil {
					return fmt.Errorf("getting GitLab connection: %w", err)
				}
				if err := glc.EnableBranchProtection(ctx, branch.Name); err != nil {
					if errors.Is(err, models.ErrProtectionAlreadyInPlace) {
						log.Printf("Branch protection already enabled for %s", branch.Name)
						continue
					}
					return fmt.Errorf("enabling branch protection: %w", err)
				}
			}
		case models.CONFIG_TAG_RULES:
			glc, err := b.getGitLabConnection(r, "")
			if err != nil {
				return fmt.Errorf("getting GitLab connection: %w", err)
			}
			if err := glc.EnableTagProtection(ctx); err != nil {
				if errors.Is(err, models.ErrProtectionAlreadyInPlace) {
					log.Printf("Tag protection already enabled")
					return nil
				}
				return fmt.Errorf("enabling tag protection: %w", err)
			}
		default:
			return fmt.Errorf("unsupported configuration: %s", config)
		}
	}

	return nil
}

// ControlPrecheck checks if prerequisites for a control are met
func (b *Backend) ControlPrecheck(*models.Repository, []*models.Branch, models.ControlConfiguration) (bool, string, models.ControlPreRemediationFn, error) {
	// For now, no special prechecks for GitLab
	return true, "", nil, nil
}
