// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package gitlab

import (
	"context"
	"errors"
	"fmt"

	"github.com/slsa-framework/source-tool/pkg/sourcetool/models"
)

// CreatePipelineMR creates a merge request to add SLSA source CI pipeline
func (b *Backend) CreatePipelineMR(r *models.Repository, branches []*models.Branch) (*models.PullRequest, error) {
	if len(branches) == 0 {
		return nil, errors.New("no branches specified")
	}

	// TODO: Implement MR creation
	// 1. Detect OIDC support
	// 2. Generate appropriate template
	// 3. Create/append to .gitlab-ci.yml
	// 4. Create merge request

	return nil, fmt.Errorf("not yet implemented")
}

// FindPipelineMR searches for an existing pipeline MR
func (b *Backend) FindPipelineMR(ctx context.Context, r *models.Repository) (*models.PullRequest, error) {
	// TODO: Implement MR search
	return nil, nil
}
