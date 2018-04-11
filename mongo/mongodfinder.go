// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package mongo

import (
	"os"
	"os/exec"
	"regexp"

	"github.com/juju/errors"
	"strconv"
)

// SearchTools represents the OS functionality we need to find the correct MongoDB executable.
// The mock for this (used in testing) is automatically generated by 'go generate' from the following line
//go:generate mockgen -package mongo -destination searchtoolsmock_test.go github.com/juju/juju/mongo SearchTools
type SearchTools interface {
	// RunCommand execs the given command, and returns the CombinedOutput, or any error that occurred.
	RunCommand(name string, arg ...string) (string, error)

	// Stat is effectively os.Stat
	Stat(string) (os.FileInfo, error)
}

// MongodFinder searches expected paths to find a version of Mongo and determine what version it is.
type MongodFinder struct {
	search SearchTools
}

// NewMongodFinder returns a type that will help search for mongod, using normal OS tools.
func NewMongodFinder() *MongodFinder {
	return &MongodFinder{
		search: osSearchTools{},
	}
}

// FindBest tries to find the mongo version that best fits what we want to use.
func (m *MongodFinder) FindBest() (string, Version, error) {
	// In Bionic and beyond (and early trusty) we just use the system mongo.
	// We only use the system mongo if it is at least Mongo 3.4
	if _, err := m.search.Stat(MongodSystemPath); err == nil {
		// We found Mongo in the system directory, check to see if the version is valid
		if v, err := m.findVersion(MongodSystemPath); err != nil {
			logger.Warningf("ignoring error trying to get version from: %q %v",
				MongodSystemPath, err)
		} else {
			return MongodSystemPath, v, nil
		}
	}
	// the system mongo is either too old, or not valid, keep trying
	return "", Version{}, errors.NotFoundf("could not find a viable 'mongod'")
}

// all mongo versions start with "db version v" and then the version is a X.Y.Z-extra
// we don't really care about the 'extra' portion of it, so we just track the rest.
var mongoVersionRegex = regexp.MustCompile(`^db version v(\d{1,9})\.(\d{1,9})(\..*)?`)

// ParseMongoVersion parses the output from "mongod --version" and returns a Version struct
func ParseMongoVersion(versionInfo string) (Version, error) {
	m := mongoVersionRegex.FindStringSubmatch(versionInfo)
	if m == nil {
		return Version{}, errors.Errorf("could not determine mongo version from:\n%s", versionInfo)
	}
	if len(m) < 3 {
		return Version{}, errors.Errorf("did not find enough version parts in:\n%s", versionInfo)
	}
	logger.Tracef("got version parts: %#v", m)
	var v Version
	var err error
	// Index '[0]' is the full matched string,
	// [1] is the Major
	// [2] is the Minor
	// [3] is the Patch to the end of the line
	if v.Major, err = strconv.Atoi(m[1]); err != nil {
		return Version{}, errors.Annotatef(err, "invalid major version: %q", versionInfo)
	}
	if v.Minor, err = strconv.Atoi(m[2]); err != nil {
		return Version{}, errors.Annotatef(err, "invalid minor version: %q", versionInfo)
	}
	if len(m) > 3 {
		// strip off the beginning '.', and make sure there is something after it
		tail := m[3]
		if len(tail) > 1 {
			v.Patch = tail[1:]
		}
	}
	return v, nil
}

func (m *MongodFinder) findVersion(path string) (Version, error) {
	out, err := m.search.RunCommand(path, "--version")
	if err != nil {
		return Version{}, errors.Trace(err)
	}
	v, err := ParseMongoVersion(out)
	if err != nil {
		return Version{}, errors.Trace(err)
	}
	if v.NewerThan(Mongo26) > 0 {
		v.StorageEngine = WiredTiger
	} else {
		v.StorageEngine = MMAPV1
	}
	return v, nil
}

type osSearchTools struct{}

func (osSearchTools) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (osSearchTools) RunCommand(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Trace(err)
	}
	return string(output), nil
}
