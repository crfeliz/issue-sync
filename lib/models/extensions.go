package models

import (
	"github.com/andygrunwald/go-jira"
	"github.com/google/go-github/github"
)

type ExtendedGithubIssue struct {
	github.Issue
	ProjectCard *github.ProjectCard
}

type ExtendedJiraIssue struct {
	jira.Issue
	AvailableStatusesByName map[string] *jira.Status
}
