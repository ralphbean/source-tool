# GitLab CI/CD Pipeline Integration Design

**Date**: 2025-11-03
**Status**: Approved
**Author**: Claude (with Ralph Bean)

## Overview

Add GitLab CI/CD pipeline integration to enable automated SLSA source provenance generation for GitLab repositories, mirroring the existing GitHub Actions implementation but adapted for GitLab's CI/CD structure and capabilities.

## Background

### Current State

The GitHub backend supports automated provenance generation via:
- GitHub Actions workflow template (`.github/workflows/compute_slsa_source.yaml`)
- `sourcetool setup controls --config=CONFIG_GEN_PROVENANCE` command
- Reusable workflow from `slsa-framework/source-actions`
- OIDC-based keyless signing via Sigstore

GitLab repositories currently:
- Can manually run `sourcetool checklevelprov` commands
- Have provenance checking capabilities (via git notes)
- Cannot automatically generate provenance on every push
- Lack CI/CD integration for SLSA source controls

### Problem

GitLab users cannot:
- Automatically generate provenance on every commit
- Easily enable SLSA source controls via CI/CD
- Achieve the same level of automation as GitHub users

## Design

### Approach

We'll implement GitLab CI/CD integration with a dual-path signing strategy:
1. **Primary**: Use GitLab's OIDC `id_tokens` with Sigstore (keyless signing)
2. **Fallback**: Use cosign with long-lived keypair (for GitLab < 15.7 or self-hosted without OIDC)

### File Structure

#### 1. `.gitlab/slsa-source.yml`

Main SLSA source provenance job template that users include in their CI:

**OIDC Version (Primary):**
```yaml
# SLSA Source Provenance Generation
# This job generates SLSA source provenance attestations and stores them in git notes
.slsa-source:
  stage: build
  image: golang:1.23
  id_tokens:
    SIGSTORE_ID_TOKEN:
      aud: sigstore
  variables:
    GIT_STRATEGY: clone
    GIT_DEPTH: 0
  before_script:
    - git config user.name "GitLab CI"
    - git config user.email "ci@gitlab.com"
  script:
    - |
      # Install sourcetool
      go install github.com/slsa-framework/source-tool@latest

      # Generate provenance based on push type
      if [ -n "$CI_COMMIT_TAG" ]; then
        # Tag push
        sourcetool checktag \
          --commit $CI_COMMIT_SHA \
          --tag_name $CI_COMMIT_TAG \
          --output_signed_bundle bundle.intoto.jsonl
      else
        # Branch push
        sourcetool checklevelprov \
          --commit $CI_COMMIT_SHA \
          --branch $CI_COMMIT_REF_NAME \
          --output_signed_bundle bundle.intoto.jsonl
      fi

      # Store provenance in git notes
      git fetch origin "refs/notes/*:refs/notes/*" || true
      git notes append -F bundle.intoto.jsonl
      git push origin "refs/notes/*"
  artifacts:
    paths:
      - bundle.intoto.jsonl
    expire_in: 30 days
  rules:
    - if: $CI_PIPELINE_SOURCE == "push"
```

**Cosign Version (Fallback for older GitLab):**
```yaml
.slsa-source:
  stage: build
  image: golang:1.23
  variables:
    GIT_STRATEGY: clone
    GIT_DEPTH: 0
  before_script:
    - git config user.name "GitLab CI"
    - git config user.email "ci@gitlab.com"
    - |
      # Install cosign
      COSIGN_VERSION=v2.2.1
      curl -LO "https://github.com/sigstore/cosign/releases/download/${COSIGN_VERSION}/cosign-linux-amd64"
      chmod +x cosign-linux-amd64
      mv cosign-linux-amd64 /usr/local/bin/cosign
  script:
    - |
      # Install sourcetool
      go install github.com/slsa-framework/source-tool@latest

      # Export private key from CI variable to file
      echo "$COSIGN_PRIVATE_KEY" > cosign.key

      # Generate provenance
      if [ -n "$CI_COMMIT_TAG" ]; then
        sourcetool checktag \
          --commit $CI_COMMIT_SHA \
          --tag_name $CI_COMMIT_TAG \
          --output_unsigned_bundle bundle.intoto.jsonl
      else
        sourcetool checklevelprov \
          --commit $CI_COMMIT_SHA \
          --branch $CI_COMMIT_REF_NAME \
          --output_unsigned_bundle bundle.intoto.jsonl
      fi

      # Sign with cosign
      cosign sign-blob --key cosign.key \
        --output-signature bundle.intoto.jsonl.sig \
        bundle.intoto.jsonl

      # Store provenance in git notes
      git fetch origin "refs/notes/*:refs/notes/*" || true
      git notes append -F bundle.intoto.jsonl
      git push origin "refs/notes/*"
  artifacts:
    paths:
      - bundle.intoto.jsonl
      - bundle.intoto.jsonl.sig
    expire_in: 30 days
  rules:
    - if: $CI_PIPELINE_SOURCE == "push"
```

#### 2. `.gitlab-ci.yml` Modification

The setup command will **append** to the existing file (not overwrite):

```yaml
include:
  - local: '.gitlab/slsa-source.yml'

slsa-source-provenance:
  extends: .slsa-source
  only:
    - branches
    - tags
```

#### 3. `.gitlab/cosign.pub` (Cosign Fallback Only)

Public key for verifying signatures when using cosign keypair:

```
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...
-----END PUBLIC KEY-----
```

### Setup Command Implementation

Extend the existing `sourcetool setup controls --config=CONFIG_GEN_PROVENANCE` to support GitLab:

#### Setup Flow

1. **Detect Platform**: Check if repository is GitLab (via hostname or API)

2. **Check OIDC Support**:
   - Query GitLab version: `GET /api/v4/version`
   - If version >= 15.7: Use OIDC approach
   - If version < 15.7: Use cosign keypair approach

3. **Generate Files**:

   **For OIDC (GitLab >= 15.7):**
   - Create `.gitlab/slsa-source.yml` with OIDC template
   - Append include to `.gitlab-ci.yml`

   **For Cosign (GitLab < 15.7):**
   - Prompt user to generate keypair: `cosign generate-key-pair`
   - Upload private key to GitLab CI/CD variables via API:
     ```
     POST /api/v4/projects/:id/variables
     {
       "key": "COSIGN_PRIVATE_KEY",
       "value": "<private-key-content>",
       "protected": true,
       "masked": true
     }
     ```
   - Create `.gitlab/cosign.pub` with public key
   - Create `.gitlab/slsa-source.yml` with cosign template
   - Append include to `.gitlab-ci.yml`

4. **Create Merge Request**:
   - Similar to GitHub PR workflow
   - Use `PullRequestManager` adapted for GitLab
   - Title: "Add SLSA Source Provenance Pipeline"
   - Body: Explanation of changes and setup instructions

### GitLab Backend Implementation

Add to `pkg/sourcetool/backends/vcs/gitlab/manage.go`:

```go
// CreatePipelineMR creates a merge request to add SLSA source CI pipeline
func (b *Backend) CreatePipelineMR(r *models.Repository, branches []*models.Branch) (*models.PullRequest, error) {
    // 1. Detect OIDC support
    hasOIDC, err := b.checkOIDCSupport()

    // 2. Generate appropriate template
    var pipelineYAML string
    var additionalFiles []*repo.PullRequestFileEntry

    if hasOIDC {
        pipelineYAML = oidcPipelineTemplate
    } else {
        // Prompt for cosign keypair
        publicKey, err := b.setupCosignKey(r)
        if err != nil {
            return nil, err
        }

        pipelineYAML = cosignPipelineTemplate
        additionalFiles = append(additionalFiles, &repo.PullRequestFileEntry{
            Path: ".gitlab/cosign.pub",
            Reader: strings.NewReader(publicKey),
        })
    }

    // 3. Read existing .gitlab-ci.yml or create new
    ciYAML, err := b.readOrCreateGitLabCI(r)

    // 4. Append include reference
    updatedCIYAML := appendInclude(ciYAML, "  - local: '.gitlab/slsa-source.yml'")

    // 5. Create MR with files
    files := []*repo.PullRequestFileEntry{
        {
            Path: ".gitlab/slsa-source.yml",
            Reader: strings.NewReader(pipelineYAML),
        },
        {
            Path: ".gitlab-ci.yml",
            Reader: strings.NewReader(updatedCIYAML),
        },
    }
    files = append(files, additionalFiles...)

    return b.createMergeRequest(r, files)
}
```

### Configuration Integration

Update `ConfigureControls` in `pkg/sourcetool/backends/vcs/gitlab/gitlab.go`:

```go
func (b *Backend) ConfigureControls(r *models.Repository, branches []*models.Branch, configs []models.ControlConfiguration) error {
    for _, config := range configs {
        switch config {
        case models.CONFIG_GEN_PROVENANCE:
            // Check if MR already exists
            mr, err := b.FindPipelineMR(ctx, r)
            if err != nil {
                return fmt.Errorf("checking for existing pipeline MR: %w", err)
            }

            if mr != nil {
                log.Printf("Pipeline MR already exists: %s", mr.URL)
                continue
            }

            // Create new MR
            if _, err := b.CreatePipelineMR(r, branches); err != nil {
                return fmt.Errorf("creating pipeline MR: %w", err)
            }
        // ... other configs
        }
    }
    return nil
}
```

## Key Differences from GitHub

| Aspect | GitHub | GitLab |
|--------|--------|--------|
| **File location** | `.github/workflows/compute_slsa_source.yaml` | `.gitlab/slsa-source.yml` + modification to `.gitlab-ci.yml` |
| **Reusable workflows** | External via `uses: slsa-framework/source-actions/...` | Local via `include:` and `extends:` |
| **OIDC tokens** | Automatic with `id-token: write` permission | Explicit `id_tokens` configuration (GitLab 15.7+) |
| **Signing** | Always Sigstore keyless | Sigstore (OIDC) or cosign keypair (fallback) |
| **Git operations** | GitHub Actions provides `GITHUB_TOKEN` | GitLab CI provides `CI_JOB_TOKEN` |
| **Variables** | `${{ github.sha }}` syntax | `$CI_COMMIT_SHA` syntax |
| **File modification** | Creates new workflow file | Appends to existing `.gitlab-ci.yml` |

## Benefits

1. **Automated Provenance**: GitLab repos can automatically generate provenance on every push
2. **SLSA Compliance**: Enables GitLab repos to achieve SLSA Source Level 3+
3. **Flexibility**: Supports both modern (OIDC) and legacy (cosign) signing methods
4. **Non-destructive**: Appends to existing CI configuration rather than replacing
5. **Consistency**: Mirrors GitHub backend capabilities

## Testing

1. **Unit Tests**:
   - Test OIDC detection logic
   - Test template generation (OIDC vs cosign)
   - Test `.gitlab-ci.yml` append logic

2. **Integration Tests**:
   - Test with gitlab.com (OIDC path)
   - Test with self-hosted GitLab 15.7+ (OIDC path)
   - Test with self-hosted GitLab < 15.7 (cosign path)
   - Verify MR creation and file content

3. **End-to-End Tests**:
   - Run full pipeline on test repository
   - Verify provenance generation
   - Verify storage in git notes

## Security Considerations

1. **OIDC Tokens**: Properly scoped to `sigstore` audience
2. **Cosign Private Keys**: Stored as protected and masked GitLab CI/CD variables
3. **Git Notes Push**: Requires CI job to have push permissions
4. **Key Generation**: User controls private key, never transmitted to sourcetool

## Implementation Checklist

- [ ] Create pipeline templates (OIDC and cosign variants)
- [ ] Implement `CreatePipelineMR` in GitLab backend
- [ ] Add OIDC detection logic
- [ ] Add cosign keypair setup workflow
- [ ] Implement `.gitlab-ci.yml` append logic
- [ ] Add `CONFIG_GEN_PROVENANCE` to `ConfigureControls`
- [ ] Add `ControlConfigurationDescr` message
- [ ] Add `getRecommendedAction` for provenance
- [ ] Write unit tests
- [ ] Write integration tests
- [ ] Update documentation

## Future Enhancements

1. **GitLab Attestations**: When GitLab's native attestation features mature, migrate from git notes
2. **Pipeline Templates**: Publish reusable GitLab CI templates in separate repo
3. **Automatic Key Rotation**: Support for rotating cosign keypairs
4. **Multi-project Pipelines**: Support for monorepos with multiple SLSA policies

## References

- [GitLab OIDC Documentation](https://docs.gitlab.com/ci/secrets/id_token_authentication/)
- [GitLab CI/CD Configuration](https://docs.gitlab.com/ee/ci/yaml/)
- [Sigstore Cosign](https://docs.sigstore.dev/cosign/overview/)
- [SLSA Source Track](https://slsa.dev/spec/v1.0/requirements#source-requirements)
