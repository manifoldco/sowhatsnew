package main

import (
	"bufio"
	"os"
	"os/exec"
	"sort"

	"github.com/pkg/errors"
)

// GitRepository is a git repository backend for sowhatsnew.
type GitRepository struct{}

// Detect determins if the current working directory is the root of a git
// repository or not.
func (GitRepository) Detect() (bool, error) {
	_, err := os.Stat(".git")
	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, errors.Wrap(err, "could not determine if .git directory exists")
	}
}

// ModifiedFiles returns the list of modified files for the provided commit
// range, lexicographically sorted, and unique.
func (GitRepository) ModifiedFiles(from, to string) ([]string, error) {
	cmd := exec.Command("git", "--no-pager", "diff", "--name-only", from, to)
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "could not read modified files")
	}

	var changedFiles []string

	scanner := bufio.NewScanner(cmdReader)
	go func() {
		for scanner.Scan() {
			changedFiles = append(changedFiles, scanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "could not read modified files")
	}

	if err := cmd.Wait(); err != nil {
		return nil, errors.Wrap(err, "could not read modified files")
	}

	sort.Strings(changedFiles)
	return changedFiles, nil
}
