// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/slsa-framework/source-tool/pkg/attest"
	"github.com/slsa-framework/source-tool/pkg/ghcontrol"
	"github.com/slsa-framework/source-tool/pkg/policy"
	"github.com/slsa-framework/source-tool/pkg/sourcetool"
	"github.com/slsa-framework/source-tool/pkg/sourcetool/models"
)

type checkLevelOpts struct {
	commitOptions
	outputVsa, outputUnsignedVsa, useLocalPolicy string
	allowMergeCommits                            bool
}

func (clo *checkLevelOpts) Validate() error {
	errs := []error{
		clo.commitOptions.Validate(),
	}

	return errors.Join(errs...)
}

func (clo *checkLevelOpts) AddFlags(cmd *cobra.Command) {
	clo.commitOptions.AddFlags(cmd)
	cmd.PersistentFlags().StringVar(&clo.outputVsa, "output_vsa", "", "The path to write a signed VSA with the determined level.")
	cmd.PersistentFlags().StringVar(&clo.outputUnsignedVsa, "output_unsigned_vsa", "", "The path to write an unsigned vsa with the determined level.")
	cmd.PersistentFlags().StringVar(&clo.useLocalPolicy, "use_local_policy", "", "UNSAFE: Use the policy at this local path instead of the official one.")
	cmd.PersistentFlags().BoolVar(&clo.allowMergeCommits, "allow-merge-commits", false, "[EXPERIMENTAL] Allow merge commits in branch.")
}

func addCheckLevel(parentCmd *cobra.Command) {
	opts := checkLevelOpts{}

	checklevelCmd := &cobra.Command{
		Use:     "checklevel",
		GroupID: "assessment",
		Short:   "Determines the SLSA Source Level of the repo",
		Long: `Determines the SLSA Source Level of the repo.

This is meant to be run within the corresponding GitHub Actions workflow.`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				if err := opts.ParseLocator(args[0]); err != nil {
					return err
				}
			}

			// Validate early the repository options to provide a more
			// useful message to the user
			if err := opts.repoOptions.Validate(); err != nil {
				return err
			}

			if err := opts.EnsureDefaults(); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}

			return doCheckLevel(&opts)
		},
	}
	opts.AddFlags(checklevelCmd)
	parentCmd.AddCommand(checklevelCmd)
}

func doCheckLevel(cla *checkLevelOpts) error {
	ctx := context.Background()

	// Get repository and branch
	repo := cla.GetRepository()
	branch := cla.GetBranch()

	// Create authenticator based on hostname
	authenticator, err := CheckAuthWithHostname(cla.hostname)
	if err != nil {
		return err
	}

	// Create sourcetool with backend abstraction
	srctool, err := sourcetool.New(
		sourcetool.WithAuthenticator(authenticator),
	)
	if err != nil {
		return err
	}

	// Get the VCS backend for this repository
	backend, err := srctool.GetVcsBackend(repo)
	if err != nil {
		return fmt.Errorf("getting VCS backend: %w", err)
	}

	// Set allow merge commits if it's a GitHub backend
	// TODO: Extend this to other backends when they support it
	if ghBackend, ok := backend.(interface{ SetAllowMergeCommits(bool) }); ok {
		ghBackend.SetAllowMergeCommits(cla.allowMergeCommits)
	}

	// Get controls at the specified commit
	commit := &models.Commit{SHA: cla.commit}
	controlStatus, err := backend.GetBranchControlsAtCommit(ctx, repo, branch, commit)
	if err != nil {
		return err
	}

	// Evaluate against policy
	pe := policy.NewPolicyEvaluator()
	pe.UseLocalPolicy = cla.useLocalPolicy
	verifiedLevels, policyPath, err := pe.EvaluateControl(ctx, repo, branch, controlStatus)
	if err != nil {
		return err
	}
	fmt.Print(verifiedLevels)

	// Create unsigned VSA
	repoUri := repo.GetHttpURL()
	fullRef := ghcontrol.BranchToFullRef(branch.Name)
	unsignedVsa, err := attest.CreateUnsignedSourceVsa(repoUri, fullRef, cla.commit, verifiedLevels, policyPath)
	if err != nil {
		return err
	}

	// Write unsigned VSA if requested
	if cla.outputUnsignedVsa != "" {
		if err := os.WriteFile(cla.outputUnsignedVsa, []byte(unsignedVsa), 0o644); err != nil { //nolint:gosec
			return err
		}
	}

	// Sign and write VSA if requested
	if cla.outputVsa != "" {
		// This will output in the sigstore bundle format.
		signedVsa, err := attest.Sign(unsignedVsa)
		if err != nil {
			return err
		}
		err = os.WriteFile(cla.outputVsa, []byte(signedVsa), 0o644) //nolint:gosec
		if err != nil {
			return err
		}
	}

	return nil
}
