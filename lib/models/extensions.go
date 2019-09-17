package models

import (
	"github.com/google/go-github/v28/github"
)

type ExtendedGithubIssue struct {
	github.Issue
	ProjectCard *github.ProjectCard
	CommitIds []string
}