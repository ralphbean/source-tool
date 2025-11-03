# GitLab API Retry Configuration Design

**Date**: 2025-11-03
**Status**: Approved
**Author**: Claude (with Ralph Bean)

## Overview

Add configurable API retry behavior to the GitLab backend to match the existing GitHub backend implementation. This improves reliability when interacting with GitLab instances (both gitlab.com and self-hosted) by allowing configuration of retry attempts for transient failures and rate limits.

## Background

### Current State

The GitHub backend (`pkg/ghcontrol`) exposes an `ApiRetries` configuration option (default: 3) that controls how many times API calls are retried on failure. The implementation uses `hashicorp/go-retryablehttp` to handle retries automatically.

The GitLab backend (`pkg/glcontrol`) creates clients using `gitlab.NewClient(token)` without any retry configuration. While the underlying GitLab client library uses `go-retryablehttp` internally with sensible defaults (5 retries, 100-400ms backoff), we don't expose any way to configure this behavior.

### Problem

Users cannot:
- Configure retry attempts to match their environment's needs
- Tune retry behavior for slow or unreliable self-hosted GitLab instances
- Achieve parity between GitHub and GitLab backend configuration APIs

## Design

### Approach

We'll implement the minimal approach: add an `ApiRetries` option to the GitLab backend's `Options` struct and pass it to the GitLab client using `WithCustomRetryMax()`. This mirrors the GitHub backend's design exactly.

### Components

#### 1. Options Structure (`pkg/glcontrol/options.go`)

Add `ApiRetries` field to match GitHub backend:

```go
type Options struct {
    AllowMergeCommits bool
    accessToken       string
    ApiRetries        uint8
}

var defaultOptions = Options{
    AllowMergeCommits: false,
    ApiRetries:        3,  // Match GitHub default
}
```

#### 2. Client Creation (`pkg/glcontrol/connection.go`)

Update `NewGitLabConnectionWithHostname()` to configure retries:

**Before:**
```go
if hostname != "" && hostname != "gitlab.com" {
    baseURL := fmt.Sprintf("https://%s/api/v4", hostname)
    client, err = gitlab.NewClient(token, gitlab.WithBaseURL(baseURL))
} else {
    client, err = gitlab.NewClient(token)
}
```

**After:**
```go
opts := defaultOptions
retryMax := int(opts.ApiRetries)

if hostname != "" && hostname != "gitlab.com" {
    baseURL := fmt.Sprintf("https://%s/api/v4", hostname)
    client, err = gitlab.NewClient(token,
        gitlab.WithBaseURL(baseURL),
        gitlab.WithCustomRetryMax(retryMax))
} else {
    client, err = gitlab.NewClient(token,
        gitlab.WithCustomRetryMax(retryMax))
}
```

#### 3. Token Authentication (`pkg/glcontrol/connection.go`)

Update `WithAuthToken()` to preserve retry configuration:

**Before:**
```go
func (glc *GitLabConnection) WithAuthToken(token string) (*GitLabConnection, error) {
    if token != "" {
        client, err := gitlab.NewClient(token)
        // ...
    }
}
```

**After:**
```go
func (glc *GitLabConnection) WithAuthToken(token string) (*GitLabConnection, error) {
    if token != "" {
        retryMax := int(glc.Options.ApiRetries)
        client, err := gitlab.NewClient(token,
            gitlab.WithCustomRetryMax(retryMax))
        // ...
    }
}
```

### Configuration

The `ApiRetries` setting controls the maximum number of retry attempts for API calls. The GitLab client library will automatically retry on:
- HTTP 429 (Rate Limit Exceeded)
- HTTP 5xx (Server Errors)

The client uses a linear jitter backoff strategy (100-400ms) and respects GitLab's `RateLimit-Reset` headers when available.

### Backward Compatibility

This change is fully backward compatible:
- Default behavior uses 3 retries (down from library default of 5)
- Existing code continues to work without changes
- No breaking API changes

## Benefits

1. **Consistency**: GitLab backend configuration API matches GitHub backend
2. **Configurability**: Users can tune retry behavior for their environment
3. **Reliability**: Better handling of transient failures and rate limits
4. **Simplicity**: Leverages existing library functionality, minimal code changes

## Testing

1. **Unit Tests**: Verify that `ApiRetries` option is properly initialized and passed to client
2. **Integration Tests**: Confirm retry behavior works with both gitlab.com and self-hosted instances
3. **Existing Tests**: All existing tests should continue to pass

## Future Enhancements

If users request more advanced configuration, we could add:
- Custom backoff timing (`RetryWaitMin`/`RetryWaitMax`)
- Custom retry logic for specific error conditions
- Retry event logging for observability

For now, the minimal approach provides the most value with the least complexity.

## Implementation Checklist

- [ ] Update `pkg/glcontrol/options.go` with `ApiRetries` field
- [ ] Update `pkg/glcontrol/connection.go` client creation
- [ ] Update `pkg/glcontrol/connection.go` WithAuthToken method
- [ ] Add/update tests
- [ ] Verify all existing tests pass
- [ ] Update documentation if needed
