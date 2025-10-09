# VCS Backend Support

The SLSA source tool supports multiple version control system (VCS) platforms, providing SLSA source attestation capabilities across different hosting providers. The tool automatically detects which platform you're using based on the repository hostname.

## Supported VCS Backends

| Backend | Status | Detection | Self-Hosted Support |
|---------|--------|-----------|---------------------|
| **GitHub** | ✅ Full Support | `github.com` or default | ❌ GitHub Enterprise not tested |
| **GitLab** | ✅ Full Support | Hostname contains `gitlab` | ✅ Yes |
| **Forgejo** | 🔮 Planned | TBD | TBD |

## Feature Coverage Matrix

### SLSA Controls

| Control | GitHub | GitLab | Forgejo | Notes |
|---------|--------|--------|---------|-------|
| **CONTINUITY_ENFORCED** | ✅ | ✅ | 🔮 | Branch force-push and deletion protection |
| **REVIEW_ENFORCED** | ✅ | ✅ | 🔮 | PR/MR approval requirements |
| **TAG_HYGIENE** | ✅ | ✅ | 🔮 | Tag protection rules |
| **PROVENANCE_AVAILABLE** | ✅ | ❌ | 🔮 | Attestation storage (GitLab: not yet implemented) |
| **POLICY_AVAILABLE** | ✅ | ✅ | 🔮 | Repository policies (stored in GitHub, applied to all) |

### Authentication Methods

| Method | GitHub | GitLab | Forgejo |
|--------|--------|--------|---------|
| **OAuth Device Flow** | ✅ | ❌ | 🔮 |
| **Personal Access Token** | ✅ (`GITHUB_TOKEN`) | ✅ (`GITLAB_TOKEN`) | 🔮 |
| **Token File** | ✅ `~/.config/slsa/sourcetool.github.token` | ✅ `~/.config/slsa/sourcetool.gitlab.token` | 🔮 |
| **Interactive Login** | ✅ `sourcetool auth login` | ❌ | 🔮 |

### CI/CD Integration

| Feature | GitHub | GitLab | Forgejo |
|---------|--------|--------|---------|
| **Workflow Templates** | ✅ GitHub Actions | ✅ GitLab CI | 🔮 |
| **Automatic Provenance** | ✅ | ⚠️ Manual setup | 🔮 |
| **Git Notes Storage** | ✅ | ✅ | 🔮 |

## Subcommand Coverage

| Command | GitHub | GitLab | Forgejo | Notes |
|---------|--------|--------|---------|-------|
| `status` | ✅ | ✅ | 🔮 | Check SLSA source level |
| `setup controls` | ✅ | ✅ | 🔮 | Configure branch/tag protection |
| `checklevel` | ✅ | ✅ | 🔮 | Determine SLSA source level |
| `checklevelprov` | ✅ | ⚠️ | 🔮 | GitLab: provenance not implemented |
| `checktag` | ✅ | ⚠️ | 🔮 | GitLab: limited support |
| `prov` | ✅ | ⚠️ | 🔮 | GitLab: provenance not implemented |
| `verifycommit` | ✅ | ⚠️ | 🔮 | GitLab: provenance not implemented |
| `audit` | ✅ | ⚠️ | 🔮 | GitLab: limited support |
| `policy create` | ✅ | ✅ | 🔮 | Creates policy (stored in GitHub) |
| `auth login` | ✅ | ❌ | 🔮 | GitHub-only OAuth flow |
| `auth whoami` | ✅ | ⚠️ | 🔮 | GitHub-only |

**Legend:**
- ✅ Full support
- ⚠️ Partial support / limitations
- ❌ Not supported
- 🔮 Planned / Future support

## Repository Detection

The tool automatically detects the VCS backend based on the repository hostname:

```bash
# GitHub (default)
sourcetool status https://github.com/owner/repo
sourcetool status owner/repo  # Assumes github.com

# GitLab
sourcetool status https://gitlab.com/group/project
sourcetool status https://gitlab.example.com/group/project  # Self-hosted

# Future: Forgejo
sourcetool status https://forge.example.com/owner/repo
```

### Detection Rules

1. If hostname contains `gitlab` → GitLab backend
2. If hostname contains `forgejo` → Forgejo backend (future)
3. Otherwise → GitHub backend (default)

---

# GitHub Backend

## Authentication

### Option 1: Interactive Login (Recommended)
```bash
sourcetool auth login
```

### Option 2: Environment Variable
```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

### Option 3: Token File
```bash
echo "ghp_xxxxxxxxxxxxxxxxxxxx" > ~/.config/slsa/sourcetool.github.token
```

## Project Identification

GitHub uses `owner/repo` format:
```bash
sourcetool status myorg/myrepo
sourcetool status https://github.com/myorg/myrepo
```

## CI/CD Integration

Use GitHub Actions workflows. See main documentation for details.

## GitHub Enterprise

GitHub Enterprise is **not currently supported**. The tool only works with `github.com` and does not support custom GitHub Enterprise instances.

To add GitHub Enterprise support, the client would need to use custom API endpoints via the `go-github` library's `WithEnterpriseURLs()` method.

---

# GitLab Backend

## Authentication

GitLab requires a personal access token with scopes: `api`, `read_repository`, `write_repository`

### Option 1: Environment Variable (Recommended)
```bash
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
```

### Option 2: Token File
```bash
echo "glpat-xxxxxxxxxxxxxxxxxxxx" > ~/.config/slsa/sourcetool.gitlab.token
```

**Token Creation:** GitLab → **User Settings** → **Access Tokens**

## Project Identification

GitLab uses `group/project` or nested group paths:

```bash
sourcetool status gitlab.com/mygroup/myproject
sourcetool status gitlab.com/mygroup/subgroup/myproject
sourcetool status https://gitlab.example.com/mygroup/myproject  # Self-hosted
```

## CI/CD Integration

Use GitLab CI/CD pipelines. Template available at `.gitlab/slsa-source-provenance.gitlab-ci.yml`

```yaml
# Include in your .gitlab-ci.yml
include:
  - local: '.gitlab/slsa-source-provenance.gitlab-ci.yml'
```

Configure `GITLAB_TOKEN` as a protected CI/CD variable in **Settings** → **CI/CD** → **Variables**

## Self-Hosted GitLab

Self-hosted GitLab instances are fully supported. Any hostname containing "gitlab" is automatically detected:

```bash
sourcetool status https://gitlab.example.com/group/project
```

---

# Forgejo Backend

🔮 **Coming Soon**

Forgejo support is planned for a future release. If you're interested in Forgejo support, please open an issue on the project repository.

---

# Cross-Platform Considerations

## Repository Policies

Repository policies are **always stored in GitHub** at `github.com/slsa-framework/source-policies`, regardless of which VCS backend you're using:

- GitHub repo policy: `policy/github.com/owner/repo/source-policy.json`
- GitLab repo policy: `policy/gitlab.com/group/project/source-policy.json`
- Self-hosted: `policy/gitlab.example.com/group/project/source-policy.json`

To check or create policies:
```bash
# View current policy status
sourcetool status <repo-url>

# Create a new policy (requires GitHub authentication)
export GITHUB_TOKEN=ghp_xxxxxxxxxxxx
sourcetool policy create <repo-url>
```

**Note:** Creating policies requires GitHub authentication since policies are stored in a GitHub repository.

## Terminology Differences

| Concept | GitHub | GitLab | Forgejo |
|---------|--------|--------|---------|
| **Code Review** | Pull Request (PR) | Merge Request (MR) | Pull Request |
| **CI/CD** | GitHub Actions | GitLab CI/CD | Forgejo Actions |
| **Project Path** | `owner/repo` | `group/project` or `group/subgroup/project` | `owner/repo` |
| **Permissions** | Repository roles | Project roles (Guest, Reporter, Developer, Maintainer, Owner) | Repository roles |

## Reference

For more information on SLSA source requirements:
- [SLSA Source Track Specification](https://slsa.dev/spec/draft/source-requirements)
- [Main Documentation](../README.md)
- [Design Documentation](./DESIGN.md)
