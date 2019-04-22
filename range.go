package main

import (
	"os"
	"strings"
)

var ciDetectors []interface {
	Detect() (string, string, bool)
}

func commitRange(from, to string) (string, string) {
	if from != "" || to != "" {
		return from, to
	}

	for _, detector := range ciDetectors {
		if from, to, ok := detector.Detect(); ok {
			return from, to
		}
	}

	return "", ""
}

type travisDetector struct{}

func (travisDetector) Detect() (string, string, bool) {
	pr := os.Getenv("TRAVIS_PULL_REQUEST")
	if pr == "" || pr == "false" {
		return "", "", false
	}

	commits := os.Getenv("TRAVIS_COMMIT_RANGE")
	parts := strings.Split(commits, "...")
	return parts[0], parts[1], true
}

func init() {
	ciDetectors = append(ciDetectors, travisDetector{})
}
