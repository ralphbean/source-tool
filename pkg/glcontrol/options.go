// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package glcontrol

type Options struct {
	// AllowMergeCommits causes the GitLab connector to reject merge
	// commits when set to false.
	AllowMergeCommits bool

	// accessToken is the token we will use to connect to the GitLab API
	accessToken string

	// ApiRetries controls the number of times we retry calls to the GitLab API
	ApiRetries uint8
}

var defaultOptions = Options{
	AllowMergeCommits: false,
	ApiRetries:        3,
}
