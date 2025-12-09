// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package glcontrol

import (
	"context"

	"github.com/slsa-framework/source-tool/pkg/sourcetool/models"
)

// ProvenanceAdapter wraps GitLabConnection to implement attest.ProvenanceConnection
// It delegates most methods directly to the embedded GitLabConnection and converts
// control status types for the two methods that have different signatures.
type ProvenanceAdapter struct {
	*GitLabConnection
}

// NewProvenanceAdapter creates a new adapter for provenance operations
func NewProvenanceAdapter(glc *GitLabConnection) *ProvenanceAdapter {
	return &ProvenanceAdapter{GitLabConnection: glc}
}

// GetBranchControlsAtCommit implements models.ProvenanceConnection
func (pa *ProvenanceAdapter) GetBranchControlsAtCommit(ctx context.Context, commit, ref string) (*models.ControlStatus, error) {
	status, err := pa.GitLabConnection.GetBranchControlsAtCommit(ctx, commit, ref)
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
	status, err := pa.GitLabConnection.GetTagControls(ctx, commit, ref)
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

// GetRepoUri implements models.ProvenanceConnection
// It wraps GitLabConnection.GetRepoUri to match the interface signature
func (pa *ProvenanceAdapter) GetRepoUri() string {
	uri, err := pa.GitLabConnection.GetRepoUri(context.Background())
	if err != nil {
		// Return empty string on error since interface doesn't allow returning error
		return ""
	}
	return uri
}
