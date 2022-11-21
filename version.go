package main

// Copyright (C) 2022 Jason E. Aten, Ph.D. All rights reserved.

import (
	"fmt"
)

var LAST_GIT_COMMIT_HASH string
var NEAREST_GIT_TAG string
var GIT_BRANCH string
var GO_VERSION string

func GetCodeVersion(programName string) string {
	return fmt.Sprintf("%s commit: %s / nearest-git-tag: %s / branch: %s / go version: %s\n",
		programName, LAST_GIT_COMMIT_HASH, NEAREST_GIT_TAG, GIT_BRANCH, GO_VERSION)
}
