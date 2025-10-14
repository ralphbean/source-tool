# GitLab Support: Remaining TODOs

This document tracks the remaining gaps between GitLab and GitHub support in the SLSA source tool.

## Priority 1: High Priority

### 1. Provenance Checking for GitLab

**Status**: ✅ Implemented

**Location**: `pkg/sourcetool/backends/vcs/gitlab/gitlab.go:84-97`

**Current State**:
- GitLab now supports provenance checking via git notes
- Implemented `GetNotesForCommit()` in `pkg/glcontrol/notes.go`
- GitLab backend checks for provenance attestations on commits
- GitLab repositories can now achieve `PROVENANCE_AVAILABLE` control
- This unblocks SLSA Source Level 3 for GitLab repos

**Implementation Details**:
1. ✅ Implemented `GetNotesForCommit()` for GitLab (similar to GitHub approach)
   - Reads attestations from `refs/notes/commits` reference
   - Supports sharded path format (`e5/73149ab...`) and top-level format
   - Handles base64-encoded content returned by GitLab API
   - Works with both gitlab.com and self-hosted instances

2. ✅ Integrated provenance checking in GitLab backend:
   - Added provenance check in `GetBranchControlsAtCommit()`
   - Detects `PROVENANCE_AVAILABLE` control when notes exist
   - Uses GitLab's `RepositoryFiles.GetFile()` API to access git notes

3. ✅ Created comprehensive test coverage:
   - `pkg/glcontrol/notes_test.go`: 6 test cases covering various scenarios
   - Tests validate sharded paths, top-level paths, 404 handling, encoding, etc.
   - All tests pass successfully

**Technical Implementation**:
```go
// New code in pkg/glcontrol/notes.go:
func (glc *GitLabConnection) GetNotesForCommit(ctx context.Context, commit string) (string, error) {
	// Reads from refs/notes/commits reference via GitLab API
	// Handles sharded and top-level path formats
	// Decodes base64 content
}

// Updated code in pkg/sourcetool/backends/vcs/gitlab/gitlab.go:
notes, err := glc.GetNotesForCommit(ctx, commit.SHA)
if notes != "" {
	activeControls.AddControl(&provenance.Control{
		Name: slsa.ProvenanceAvailable.String(),
	})
}
```

**Acceptance Criteria**:
- [x] GitLab repositories can store provenance attestations (using git notes)
- [x] `GetBranchControlsAtCommit()` can detect `PROVENANCE_AVAILABLE` for GitLab repos
- [x] Attestations work on both gitlab.com and self-hosted GitLab
- [x] Tests cover GitLab provenance scenarios
- [x] GitLab repos can achieve SLSA Source Level 3+

**Completed**: January 2025

---

## Priority 2: Medium Priority

### 2. Tag Controls Implementation

**Status**: 🔴 Blocked for GitLab, 🟢 Possible for GitHub

**Location**:
- `pkg/sourcetool/backends/vcs/gitlab/gitlab.go:137-139`
- `pkg/sourcetool/backends/vcs/github/github.go:172-174`

**Current State**:
- `GetTagControls()` returns "not yet implemented" in both backends
- Tag-level SLSA controls cannot be validated
- `TAG_HYGIENE` control validation is incomplete

**GitLab Blocker** 🚫:

After investigating GitLab's API capabilities, we've determined that GitLab **cannot currently support SLSA Source Level 2+ tag verification** due to missing API features:

**SLSA Requirement**: Tags must be prevented from being **moved** (updated to different commits) and **deleted**, with verifiable "time in force" for these protections.

**GitLab Gaps**:
1. **Tag Update Prevention**: GitLab has no mechanism to prevent tags from being force-pushed/updated to point to different commits
   - Protected Tags API only controls WHO can CREATE tags
   - No `deny_update_tag` or `deny_force_push_tag` push rule exists
   - Tags can be moved with `git push --force origin v1.0.0`

2. **Timestamp Verification**: Cannot determine when tag protections were enabled
   - Protected Tags API has no `created_at` or `updated_at` fields
   - Push Rules API has `created_at` but only covers deletion (not updates)
   - No audit events for protected tag changes (tracked in GitLab issue #268122)

3. **Partial Protection**: While `deny_delete_tag` push rule prevents deletion, it doesn't prevent updates
   - Can verify deletion prevention ✅
   - Cannot verify update prevention ❌
   - SLSA requires BOTH for tag immutability

**Upstream Issue**: https://gitlab.com/gitlab-org/gitlab/-/issues/579382
- Requesting `deny_update_tag` push rule
- Requesting timestamps on Protected Tags API
- Requesting audit events for tag protection changes

**What's Needed for GitLab**:
1. GitLab to implement tag update/move prevention mechanism
2. GitLab to add timestamps to Protected Tags API
3. GitLab to add audit events for tag protection changes (#268122)

**What's Needed for GitHub**:
1. Implement `GetTagControls()` in `pkg/ghcontrol/checklevel.go`
   - Call existing `computeTagHygieneControl()`
   - Return `*GhControlStatus` with tag hygiene state
2. Wire up GitHub backend to call `ghcontrol.GetTagControls()`
3. Add tests for tag protection scenarios

**GitHub Status**: ✅ GitHub's API supports full SLSA tag verification
- Tag rulesets include Update, Deletion, NonFastForward rules
- Rulesets have `UpdatedAt` timestamps
- Can verify both prevention mechanisms and time in force

**Acceptance Criteria**:
- [x] ~~`GetTagControls()` implemented for GitLab~~ **Blocked on upstream GitLab issue #579382**
- [ ] `GetTagControls()` implemented for GitHub
- [ ] Returns accurate `TAG_HYGIENE` control status (GitHub only)
- [ ] Includes timestamp for when protection was enabled (GitHub only)
- [ ] Tests cover various tag protection scenarios (GitHub only)
- [ ] Documentation clearly states GitLab limitation

**Estimated Effort**:
- GitHub implementation: Small (1 day)
- GitLab implementation: Blocked on upstream (months/years)

---

### 3. Enhanced Merge Request Approval Rules

**Status**: 🟡 Partially Implemented

**Location**: `pkg/glcontrol/checklevel.go` (GitLab API integration)

**Current State**:
- Basic `REVIEW_ENFORCED` control checking exists
- GitLab approval rules are fetched via API
- Advanced approval requirements may not be fully validated

**What's Needed**:
1. **Comprehensive Approval Rule Validation**:
   - Minimum number of approvals required
   - Eligible approvers (specific users, groups, or roles)
   - Code owner approvals
   - Prevent approval by commit author
   - Prevent approval by merge request author

2. **GitLab-Specific Features**:
   - Approval rules per merge request target branch
   - Protected branch approval requirements
   - Security approval rules (GitLab Premium/Ultimate)
   - License scanning approval rules

3. **Mapping to SLSA Controls**:
   - Document which approval settings satisfy `REVIEW_ENFORCED`
   - Handle different GitLab tiers (CE vs EE features)
   - Provide clear messaging about configuration gaps

**API Endpoints**:
- `GET /api/v4/projects/:id/approval_rules`
- `GET /api/v4/projects/:id/protected_branches/:name`

**Acceptance Criteria**:
- [ ] All relevant approval settings are checked
- [ ] GitLab EE features are properly detected and handled
- [ ] Clear documentation of approval requirements for SLSA compliance
- [ ] Tests cover various approval rule configurations
- [ ] Helpful error messages for misconfigured approval rules

**Estimated Effort**: Small-Medium (1-2 days)

---

## Priority 3: Lower Priority

### 4. Self-Hosted GitLab API Rate Limiting

**Status**: 🟡 Basic Support Exists

**Current State**:
- GitLab API calls are made without rate limit handling
- No retry logic for transient failures
- Self-hosted instances may have different rate limits than gitlab.com

**What's Needed**:
1. **Rate Limit Handling**:
   - Parse `RateLimit-*` headers from GitLab API responses
   - Implement exponential backoff for 429 responses
   - Queue requests when approaching rate limits

2. **Retry Logic**:
   - Retry on transient network errors
   - Retry on 5xx server errors
   - Configurable retry attempts and backoff strategy

3. **Instance Detection**:
   - Detect GitLab version and tier (CE vs EE)
   - Adjust behavior based on available features
   - Handle API differences between versions

4. **Configuration**:
   - Allow users to configure rate limit thresholds
   - Support custom retry strategies
   - Respect instance-specific rate limits

**Acceptance Criteria**:
- [ ] Graceful handling of rate limit responses
- [ ] Automatic retry with exponential backoff
- [ ] Detection of GitLab version and tier
- [ ] Configuration options for rate limiting behavior
- [ ] Tests with mocked rate limit scenarios

**Estimated Effort**: Small (1-2 days)

---

### 5. GitLab CI/CD Pipeline Integration

**Status**: 🔴 Not Implemented

**Location**: N/A (new feature)

**Current State**:
- No integration with GitLab CI/CD for provenance generation
- GitHub Actions workflow exists for GitHub repos
- GitLab repos cannot automatically generate provenance

**What's Needed**:
1. **GitLab CI Template**:
   - Create `.gitlab-ci.yml` template for provenance generation
   - Similar to existing GitHub Actions workflow
   - Support both gitlab.com and self-hosted runners

2. **Provenance Generation in CI**:
   - Run on every commit/merge request
   - Generate SLSA provenance attestation
   - Store attestation in git notes or GitLab Attestations

3. **Configuration Command**:
   - Extend `sourcetool setup controls --config=CONFIG_GEN_PROVENANCE`
   - Generate GitLab CI configuration for provenance
   - Create merge request with the configuration

4. **Documentation**:
   - Document how to enable provenance in GitLab CI
   - Explain differences from GitHub Actions approach
   - Provide troubleshooting guide

**Acceptance Criteria**:
- [ ] GitLab CI template for provenance generation
- [ ] `setup controls` command generates GitLab CI config
- [ ] Automated provenance generation on GitLab repos
- [ ] Documentation and examples
- [ ] Integration tests with GitLab CI

**Estimated Effort**: Large (4-6 days)

---

## Priority 4: Nice-to-Have

### 6. GitLab Group-Level Policy Support

**Status**: 🔴 Not Implemented

**Current State**:
- Policies are repository-specific
- No support for organization/group-level policies
- Each repo needs individual policy configuration

**What's Needed**:
1. Support GitLab group-level compliance frameworks
2. Inherit policies from parent groups
3. Override group policies at project level
4. Document group policy inheritance model

**Estimated Effort**: Medium (2-3 days)

---

### 7. GitLab Compliance Pipeline Integration

**Status**: 🔴 Not Implemented

**Current State**:
- No integration with GitLab compliance pipelines
- GitLab EE compliance features not utilized

**What's Needed**:
1. Integrate with GitLab compliance pipelines
2. Generate compliance reports for SLSA controls
3. Support compliance frameworks
4. Document compliance pipeline configuration

**Estimated Effort**: Medium-Large (3-4 days)

---

## Implementation Order Recommendation

Based on priority and dependencies, recommended implementation order:

1. **Provenance Checking for GitLab** (Priority 1, Item 1)
   - Blocking for SLSA Level 3+ on GitLab
   - Required before GitLab CI/CD integration makes sense
   - Highest impact

2. **Tag Controls Implementation** (Priority 2, Item 2)
   - Benefits both GitHub and GitLab
   - Relatively self-contained
   - Completes tag-level SLSA support

3. **GitLab CI/CD Pipeline Integration** (Priority 3, Item 5)
   - Depends on provenance checking being complete
   - Enables automated provenance generation
   - Significant user value

4. **Enhanced Merge Request Approval Rules** (Priority 2, Item 3)
   - Improves existing functionality
   - Relatively small scope
   - Fills gaps in current implementation

5. **Self-Hosted GitLab API Rate Limiting** (Priority 3, Item 4)
   - Improves reliability
   - Can be done incrementally
   - Lower priority for initial functionality

6. **Nice-to-Have Features** (Priority 4, Items 6-7)
   - Implement based on user demand
   - Not blocking for core functionality

---

## Contributing

When working on these items:
1. Create tests first (TDD approach)
2. Update documentation in parallel with implementation
3. Consider backward compatibility with existing GitLab support
4. Test against both gitlab.com and self-hosted instances
5. Follow the existing code patterns from GitHub backend

## References

- [GitLab API Documentation](https://docs.gitlab.com/ee/api/)
- [SLSA Specification](https://slsa.dev/spec/v1.0/)
- [Source Track Requirements](https://slsa.dev/spec/v1.0/requirements)
- [GitLab Attestations](https://docs.gitlab.com/ee/ci/yaml/artifacts_reports.html#artifactsreportsattestation)
