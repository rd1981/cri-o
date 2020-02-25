// This tools automatically finds the latest CRI-O development version for the
// master or release branches. A development version is the next possible version
// suffixed with `-dev`. If we're on an actual tag, then we use this one because
// we assume that our target is to build a release version from that tag.
//
// In case of any error this tool will return "unknown" as latest tag.
package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/blang/semver"
	"github.com/sirupsen/logrus"
	"k8s.io/release/pkg/command"
	kgit "k8s.io/release/pkg/git"
	"k8s.io/release/pkg/util"
)

const (
	fullVersion = "v[0-9].[0-9]+.[0-9]$"
	remote      = "https://github.com/cri-o/cri-o"
	git         = "git"
	grep        = "grep"
	tail        = "tail"
	sort        = "sort"
)

func main() {
	// Disable any logs
	logrus.SetOutput(ioutil.Discard)

	// The result tag
	tag := "0.0.0-unknown"

	// Check if everything is in $PATH and we're running git >= 2.0.0
	if !command.Available(git, grep, tail, sort) ||
		command.
			New(git, "version").
			Pipe(grep, "-q", "git version 2").
			RunSilentSuccess() != nil {
		fmt.Print(tag)
		return
	}

	// If we're directly on a tag, then we use this one as highest priority
	describeRes, err := command.New(
		git, "describe", "--tags", "--abbrev=0", "--exact-match",
	).RunSilentSuccessOutput()
	if err != nil {
		// Scope the version to `major.minor` on release branches
		branch := kgit.Master
		branchRes, err := command.New(
			git, "rev-parse", "--abbrev-ref", "HEAD",
		).RunSilentSuccessOutput()
		if err == nil {
			branch = strings.TrimSpace(branchRes.Output())
		}
		versionScope := ""
		if strings.HasPrefix(branch, "release-") {
			versionScope = strings.TrimPrefix(branch, "release-")
		}

		// Assume we have internet access, so we use the tag from the upstream
		// remote
		lsRemoteRes, err := command.
			New(git, "ls-remote", "--sort=v:refname", "--tags", remote).
			Pipe(grep, versionScope).
			Pipe(grep, "-Eo", fullVersion).
			Pipe(tail, "-1").
			RunSilentSuccessOutput()

		if err != nil {
			// Fallback to the local git repository, if ls-remote failed
			localTagRes, err := command.
				New(git, "tag").
				Pipe(grep, versionScope).
				Pipe(grep, "-Eo", fullVersion).
				Pipe(sort, "-V").
				Pipe(tail, "-1").
				RunSilentSuccessOutput()

			if err == nil {
				tag = incVersion(localTagRes.Output(), branch)
			}
		} else {
			tag = incVersion(lsRemoteRes.Output(), branch)
		}
	} else {
		tag = describeRes.Output()
	}
	tag = strings.TrimSpace(tag)

	fmt.Print(util.TrimTagPrefix(tag))
}

func incVersion(tag, branch string) string {
	sv, err := util.TagStringToSemver(strings.TrimSpace(tag))
	if err != nil {
		panic(err)
	}
	// Release branches get the next patch
	if kgit.IsReleaseBranch(branch) {
		sv.Patch++
	} else {
		sv.Minor++
	}
	sv.Pre = []semver.PRVersion{{VersionStr: "dev"}}

	return sv.String()
}