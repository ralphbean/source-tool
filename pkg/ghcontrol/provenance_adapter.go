// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package ghcontrol

import (
	"context"

	"github.com/slsa-framework/source-tool/pkg/sourcetool/models"
)

// ProvenanceAdapter wraps GitHubConnection to implement attest.ProvenanceConnection
// It delegates most methods directly to the embedded GitHubConnection and converts
// control status types for the two methods that have different signatures.
type ProvenanceAdapter struct {
	*GitHubConnection
}

// NewProvenanceAdapter creates a new adapter for provenance operations
func NewProvenanceAdapter(ghc *GitHubConnection) *ProvenanceAdapter {
	return &ProvenanceAdapter{GitHubConnection: ghc}
}

// GetBranchControlsAtCommit implements models.ProvenanceConnection
func (pa *ProvenanceAdapter) GetBranchControlsAtCommit(ctx context.Context, commit, ref string) (*models.ControlStatus, error) {
	status, err := pa.GitHubConnection.GetBranchControlsAtCommit(ctx, commit, ref)
	if err != nil {
		return nil, err
	}
	return &models.ControlStatus{
		CommitPushTime: status.CommitPushTime,
		ActorLogin:     status.ActorLogin,
		ActivityType:   status.ActivityType,
		Controls:       status.Controls,
	}, nil
}

// GetTagControls implements models.ProvenanceConnection
func (pa *ProvenanceAdapter) GetTagControls(ctx context.Context, commit, ref string) (*models.ControlStatus, error) {
	status, err := pa.GitHubConnection.GetTagControls(ctx, commit, ref)
	if err != nil {
		return nil, err
	}
	return &models.ControlStatus{
		CommitPushTime: status.CommitPushTime,
		ActorLogin:     status.ActorLogin,
		ActivityType:   status.ActivityType,
		Controls:       status.Controls,
	}, nil
}
