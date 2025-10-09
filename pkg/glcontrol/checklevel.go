// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package glcontrol

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/slsa-framework/source-tool/pkg/provenance"
	"github.com/slsa-framework/source-tool/pkg/slsa"
	"github.com/slsa-framework/source-tool/pkg/sourcetool/models"
)

type GLControlStatus struct {
	// The time the commit we're evaluating was pushed.
	CommitPushTime time.Time
	// The actor that pushed the commit.
	ActorLogin string
	// The type of activity that created the commit.
	ActivityType string
	// The controls that are enabled according to the GitLab API.
	Controls slsa.Controls
}

// AddControl adds the control, but only if it existed when the commit was pushed.
func (cs *GLControlStatus) AddControl(newControls ...*provenance.Control) {
	for _, newControl := range newControls {
		if newControl != nil && cs.CommitPushTime.After(newControl.GetSince().AsTime()) {
			cs.Controls.AddControl(newControl)
		}
	}
}

// GetBranchControls returns a list of the controls enabled at present for a branch.
func (glc *GitLabConnection) GetBranchControls(ctx context.Context, ref string) (*slsa.Controls, error) {
	branchName := GetBranchFromRef(ref)
	if branchName == "" {
		return nil, fmt.Errorf("ref %s is not a branch", ref)
	}

	controls := &slsa.Controls{}

	// Get protected branch settings
	protectedBranch, _, err := glc.Client().ProtectedBranches.GetProtectedBranch(glc.projectID, branchName)
	if err != nil {
		// Branch might not be protected, which is fine
		if !strings.Contains(err.Error(), "404") {
			return nil, fmt.Errorf("getting protected branch: %w", err)
		}
		// Not protected, return empty controls
		return controls, nil
	}

	// Compute continuity control
	continuityControl, err := glc.computeContinuityControl(ctx, protectedBranch)
	if err != nil {
		return nil, fmt.Errorf("could not populate ContinuityControl: %w", err)
	}
	controls.AddControl(continuityControl)

	// Compute review control
	reviewControl, err := glc.computeReviewControl(ctx, protectedBranch)
	if err != nil {
		return nil, fmt.Errorf("could not populate ReviewControl: %w", err)
	}
	controls.AddControl(reviewControl)

	// Check required pipelines (equivalent to required status checks)
	requiredPipelineControls, err := glc.computeRequiredPipelines(ctx, branchName)
	if err != nil {
		return nil, fmt.Errorf("could not populate RequiredPipelines: %w", err)
	}
	controls.AddControl(requiredPipelineControls...)

	// Check tag hygiene
	tagHygieneControl, err := glc.computeTagHygieneControl(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not populate TagHygieneControl: %w", err)
	}
	controls.AddControl(tagHygieneControl)

	return controls, nil
}

// GetBranchControlsAtCommit determines the controls that are in place for a branch at a specific commit.
func (glc *GitLabConnection) GetBranchControlsAtCommit(ctx context.Context, commit, ref string) (*GLControlStatus, error) {
	// Get the commit info to determine when it was pushed
	commitInfo, _, err := glc.Client().Commits.GetCommit(glc.projectID, commit, nil)
	if err != nil {
		return nil, fmt.Errorf("getting commit info: %w", err)
	}

	controlStatus := GLControlStatus{
		CommitPushTime: *commitInfo.CommittedDate,
		ActorLogin:     commitInfo.CommitterName,
		ActivityType:   "push", // GitLab doesn't distinguish as granularly
		Controls:       slsa.Controls{},
	}

	activeControls, err := glc.GetBranchControls(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("reading active controls: %w", err)
	}

	// Add the controls to the control status object.
	for _, c := range *activeControls {
		controlStatus.AddControl(c)
	}

	return &controlStatus, nil
}

// computeContinuityControl checks for force push and delete protection.
func (glc *GitLabConnection) computeContinuityControl(ctx context.Context, pb *gitlab.ProtectedBranch) (*provenance.Control, error) {
	// In GitLab, we check if:
	// 1. The branch is protected (which prevents deletion by default)
	// 2. AllowForcePush is false (which prevents force pushes)

	if pb == nil {
		return nil, nil
	}

	// If AllowForcePush is true, continuity is not enforced
	if pb.AllowForcePush {
		log.Printf("Branch allows force push, cannot be L2+")
		return nil, nil
	}

	// Get project to check when protection was last updated
	// For now, we'll use a conservative approach and set the since time to now
	// In a real implementation, we'd need to track when protections were added
	since := time.Now()

	return &provenance.Control{
		Name:  string(slsa.ContinuityEnforced),
		Since: timestamppb.New(since),
	}, nil
}

// computeReviewControl checks for merge request approval requirements.
func (glc *GitLabConnection) computeReviewControl(ctx context.Context, pb *gitlab.ProtectedBranch) (*provenance.Control, error) {
	if pb == nil {
		return nil, nil
	}

	// Check if merge requests are required and approvals are configured
	// In GitLab, this is controlled by the MergeAccessLevels
	// We need to ensure only certain roles can merge and approvals are required

	// Get project approval settings
	approvalRules, _, err := glc.Client().Projects.GetProjectApprovalRules(glc.projectID, nil)
	if err != nil {
		// Approval rules might not be configured
		return nil, nil
	}

	// Check if there's at least one approval rule with required approvals
	hasApprovalRule := false
	for _, rule := range approvalRules {
		if rule.ApprovalsRequired > 0 {
			hasApprovalRule = true
			break
		}
	}

	if !hasApprovalRule {
		return nil, nil
	}

	// Conservative approach: use current time
	since := time.Now()

	return &provenance.Control{
		Name:  slsa.ReviewEnforced.String(),
		Since: timestamppb.New(since),
	}, nil
}

// computeRequiredPipelines checks for required pipeline success.
func (glc *GitLabConnection) computeRequiredPipelines(ctx context.Context, branchName string) ([]*provenance.Control, error) {
	// Get the project to check pipeline settings
	project, _, err := glc.Client().Projects.GetProject(glc.projectID, nil)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}

	controls := []*provenance.Control{}

	// Check if pipelines must succeed for merge requests
	if project.OnlyAllowMergeIfPipelineSucceeds {
		controls = append(controls, &provenance.Control{
			Name:  "GL_REQUIRED_PIPELINE",
			Since: timestamppb.New(time.Now()),
		})
	}

	return controls, nil
}

// computeTagHygieneControl checks for tag protection.
func (glc *GitLabConnection) computeTagHygieneControl(ctx context.Context) (*provenance.Control, error) {
	// Get all protected tags
	protectedTags, _, err := glc.Client().ProtectedTags.ListProtectedTags(glc.projectID, nil)
	if err != nil {
		return nil, fmt.Errorf("getting protected tags: %w", err)
	}

	// Check if there's a wildcard protection (e.g., "*" or "v*")
	hasWildcardProtection := false
	for _, tag := range protectedTags {
		if tag.Name == "*" || strings.Contains(tag.Name, "*") {
			hasWildcardProtection = true
			break
		}
	}

	if !hasWildcardProtection {
		return nil, nil
	}

	// Conservative approach: use current time
	since := time.Now()

	return &provenance.Control{
		Name:  slsa.TagHygiene.String(),
		Since: timestamppb.New(since),
	}, nil
}

// GetTagControls returns controls for tag protection.
func (glc *GitLabConnection) GetTagControls(ctx context.Context, commit, ref string) (*GLControlStatus, error) {
	controlStatus := GLControlStatus{
		CommitPushTime: time.Now(),
		Controls:       slsa.Controls{},
	}

	tagHygieneControl, err := glc.computeTagHygieneControl(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not populate TagHygieneControl: %w", err)
	}
	controlStatus.AddControl(tagHygieneControl)

	return &controlStatus, nil
}

// GetBranchFromRef extracts the branch name from a full Git ref.
func GetBranchFromRef(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

// EnableBranchProtection adds branch protection if not already present.
func (glc *GitLabConnection) EnableBranchProtection(ctx context.Context, branchName string) error {
	// Check if branch is already protected
	_, _, err := glc.Client().ProtectedBranches.GetProtectedBranch(glc.projectID, branchName)
	if err == nil {
		// Already protected
		return models.ErrProtectionAlreadyInPlace
	}

	// Protect the branch
	_, _, err = glc.Client().ProtectedBranches.ProtectRepositoryBranches(
		glc.projectID,
		&gitlab.ProtectRepositoryBranchesOptions{
			Name:             &branchName,
			AllowForcePush:   gitlab.Ptr(false),
			PushAccessLevel:  gitlab.Ptr(gitlab.MaintainerPermissions),
			MergeAccessLevel: gitlab.Ptr(gitlab.DeveloperPermissions),
		},
	)
	if err != nil {
		return fmt.Errorf("protecting branch: %w", err)
	}

	return nil
}

// EnableTagProtection adds tag protection for all tags.
func (glc *GitLabConnection) EnableTagProtection(ctx context.Context) error {
	// Check if wildcard protection exists
	protectedTags, _, err := glc.Client().ProtectedTags.ListProtectedTags(glc.projectID, nil)
	if err != nil {
		return fmt.Errorf("listing protected tags: %w", err)
	}

	for _, tag := range protectedTags {
		if tag.Name == "*" {
			// Already protected
			return models.ErrProtectionAlreadyInPlace
		}
	}

	// Protect all tags
	tagName := "*"
	_, _, err = glc.Client().ProtectedTags.ProtectRepositoryTags(
		glc.projectID,
		&gitlab.ProtectRepositoryTagsOptions{
			Name:              &tagName,
			CreateAccessLevel: gitlab.Ptr(gitlab.MaintainerPermissions),
		},
	)
	if err != nil {
		return fmt.Errorf("protecting tags: %w", err)
	}

	return nil
}
