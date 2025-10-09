// SPDX-FileCopyrightText: Copyright 2025 The SLSA Authors
// SPDX-License-Identifier: Apache-2.0

package glcontrol

type Options struct {
	AllowMergeCommits bool
}

var defaultOptions = Options{
	AllowMergeCommits: false,
}
