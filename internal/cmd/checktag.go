// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/slsa-framework/source-tool/pkg/attest"
	"github.com/slsa-framework/source-tool/pkg/ghcontrol"
	"github.com/slsa-framework/source-tool/pkg/glcontrol"
	"github.com/slsa-framework/source-tool/pkg/policy"
	"github.com/slsa-framework/source-tool/pkg/sourcetool/models"
)

type checkTagOptions struct {
	repoOptions
	verifierOptions
	commit             string
	tagName            string
	actor              string
	outputSignedBundle string
	useLocalPolicy     string
	vsaRetries         uint8
}

func (cto *checkTagOptions) Validate() error {
	errs := []error{
		cto.repoOptions.Validate(),
		cto.verifierOptions.Validate(),
	}
	return errors.Join(errs...)
}

func (cto *checkTagOptions) AddFlags(cmd *cobra.Command) {
	cto.repoOptions.AddFlags(cmd)
	cto.verifierOptions.AddFlags(cmd)
	cmd.PersistentFlags().StringVar(&cto.commit, "commit", "", "The commit to check - required.")
	cmd.PersistentFlags().StringVar(&cto.tagName, "tag_name", "", "The name of the new tag - required.")
	cmd.PersistentFlags().StringVar(&cto.actor, "actor", "", "The username of the actor that pushed the tag.")
	cmd.PersistentFlags().StringVar(&cto.outputSignedBundle, "output_signed_bundle", "", "The path to write a bundle of signed attestations.")
	cmd.PersistentFlags().StringVar(&cto.useLocalPolicy, "use_local_policy", "", "UNSAFE: Use the policy at this local path instead of the official one.")
	cmd.PersistentFlags().Uint8Var(&cto.vsaRetries, "retries", 3, "Number of times to retry fetching the commit's VSA")
}

func addCheckTag(parentCmd *cobra.Command) {
	opts := &checkTagOptions{}

	checktagCmd := &cobra.Command{
		Use:     "checktag",
		GroupID: "assessment",
		Short:   "Checks to see if the tag operation should be allowed and issues a VSA",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doCheckTag(opts)
		},
	}

	opts.AddFlags(checktagCmd)
	parentCmd.AddCommand(checktagCmd)
}

func doCheckTag(args *checkTagOptions) error {
	ctx := context.Background()
	repo := args.GetRepository()

	tagRef := ghcontrol.TagToFullRef(args.tagName)

	// Create the appropriate VCS connection based on hostname
	var connection models.ProvenanceConnection
	var repoUri string
	var fullRef string
	var err error

	if repo != nil && repo.Hostname != "" && strings.Contains(strings.ToLower(repo.Hostname), "gitlab") {
		// GitLab repository
		glc, err := glcontrol.NewGitLabConnectionWithHostname(repo.Path, tagRef, repo.Hostname)
		if err != nil {
			return fmt.Errorf("creating GitLab connection: %w", err)
		}
		connection = glcontrol.NewProvenanceAdapter(glc)
		repoUri = connection.GetRepoUri()
		fullRef = connection.GetFullRef()
	} else {
		// GitHub repository (default)
		ghconnection := ghcontrol.NewGhConnection(args.owner, args.repository, tagRef).WithAuthToken(githubToken)
		connection = ghcontrol.NewProvenanceAdapter(ghconnection)
		repoUri = ghconnection.GetRepoUri()
		fullRef = ghconnection.GetFullRef()
	}

	verifier := getVerifier(&args.verifierOptions)

	// Create tag provenance.
	pa := attest.NewProvenanceAttestor(connection, verifier)
	pa.Options.VsaRetries = args.vsaRetries // Retry fetching the commit's VSA

	prov, err := pa.CreateTagProvenance(ctx, args.commit, tagRef, args.actor)
	if err != nil {
		return fmt.Errorf("creating tag provenance metadata: %w", err)
	}

	// check p against policy
	pe := policy.NewPolicyEvaluator()
	pe.UseLocalPolicy = args.useLocalPolicy
	verifiedLevels, policyPath, err := pe.EvaluateTagProv(ctx, args.GetRepository(), prov)
	if err != nil {
		return fmt.Errorf("evaluating the tag provenance metadata: %w", err)
	}

	// create vsa
	unsignedVsa, err := attest.CreateUnsignedSourceVsa(repoUri, fullRef, args.commit, verifiedLevels, policyPath)
	if err != nil {
		return err
	}

	unsignedProv, err := protojson.Marshal(prov)
	if err != nil {
		return err
	}

	if args.outputSignedBundle != "" {
		f, err := os.OpenFile(args.outputSignedBundle, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644) //nolint:gosec
		if err != nil {
			return err
		}
		defer f.Close() //nolint:errcheck

		signedProv, err := attest.Sign(string(unsignedProv))
		if err != nil {
			return err
		}

		signedVsa, err := attest.Sign(unsignedVsa)
		if err != nil {
			return err
		}

		if _, err := f.WriteString(signedProv + "\n" + signedVsa + "\n"); err != nil {
			return fmt.Errorf("writing bundledata: %w", err)
		}
	} else {
		log.Printf("unsigned prov: %s\n", unsignedProv)
		log.Printf("unsigned vsa: %s\n", unsignedVsa)
	}
	fmt.Print(verifiedLevels)
	return nil
}
