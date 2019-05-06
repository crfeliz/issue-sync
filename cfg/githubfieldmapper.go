package cfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/andygrunwald/go-jira"
	"github.com/google/go-github/github"
	"strings"
	"time"
)

type FieldMapper interface {
	MapFields(issue *github.Issue) jira.IssueFields

	GetFieldValue(jIssue jira.Issue, fieldKey FieldKey) (interface{}, error)

	// getFieldIDs requests the metadata of every issue field in the JIRA
	// project, and saves the IDs of the custom fields used by issue-sync.
	GetFieldIDs(client jira.Client) (map[FieldKey]string, error)
}

type DefaultFieldMapper struct {
	Config *Config
}

type JsonFieldMapper struct {
	Config *Config
}

func (m DefaultFieldMapper) GetFieldValue(jIssue jira.Issue, fieldKey FieldKey) (interface{}, error) {
	jiraFieldValue, err := jIssue.Fields.Unknowns.String(m.Config.GetFieldKey(fieldKey))
	return jiraFieldValue, err
}

func (m DefaultFieldMapper) MapFields(issue *github.Issue) jira.IssueFields {
	fields := jira.IssueFields{
		Type: jira.IssueType{
			Name: "Task", // TODO: Determine issue type
		},
		Project:     m.Config.GetProject(),
		Summary:     issue.GetTitle(),
		Description: issue.GetBody(),
		Unknowns:    map[string]interface{}{},
	}

	fields.Unknowns[m.Config.GetFieldKey(GitHubID)] = issue.GetID()
	fields.Unknowns[m.Config.GetFieldKey(GitHubNumber)] = issue.GetNumber()
	fields.Unknowns[m.Config.GetFieldKey(GitHubStatus)] = issue.GetState()
	fields.Unknowns[m.Config.GetFieldKey(GitHubReporter)] = issue.User.GetLogin()

	strs := make([]string, len(issue.Labels))
	for i, v := range issue.Labels {
		strs[i] = *v.Name
	}
	fields.Unknowns[m.Config.GetFieldKey(GitHubLabels)] = strings.Join(strs, ",")

	fields.Unknowns[m.Config.GetFieldKey(LastISUpdate)] = time.Now().Format(DateFormat)

	return fields
}

func (m DefaultFieldMapper) GetFieldIDs(client jira.Client) (map[FieldKey]string, error) {
	m.Config.log.Debug("Collecting field IDs.")
	req, err := client.NewRequest("GET", "/rest/api/2/field", nil)
	if err != nil {
		return map[FieldKey]string{}, err
	}
	jFields := new([]jiraField)

	_, err = client.Do(req, jFields)
	if err != nil {
		return map[FieldKey]string{}, err
	}

	fieldIDs := map[FieldKey]string{}

	for _, field := range *jFields {
		switch field.Name {
		case "GitHub ID":
			fieldIDs[GitHubID] = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Number":
			fieldIDs[GitHubNumber] = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Labels":
			fieldIDs[GitHubLabels] = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Status":
			fieldIDs[GitHubStatus] = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Reporter":
			fieldIDs[GitHubReporter] = fmt.Sprint(field.Schema.CustomID)
		case "Last Issue-Sync Update":
			fieldIDs[LastISUpdate] = fmt.Sprint(field.Schema.CustomID)
		}
	}

	_, ok := fieldIDs[GitHubID]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'GitHub ID' custom field; check that it is named correctly")
	}

	_, ok = fieldIDs[GitHubNumber]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'GitHub Number' custom field; check that it is named correctly")
	}

	_, ok = fieldIDs[GitHubLabels]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'Github Labels' custom field; check that it is named correctly")
	}

	_, ok = fieldIDs[GitHubStatus]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'Github Status' custom field; check that it is named correctly")
	}

	_, ok = fieldIDs[GitHubReporter]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'Github Reporter' custom field; check that it is named correctly")
	}

	_, ok = fieldIDs[LastISUpdate]
	if !ok  {
		return fieldIDs, errors.New("could not find ID of 'Last Issue-Sync Update' custom field; check that it is named correctly")
	}

	m.Config.log.Debug("All fields have been checked.")

	return fieldIDs, nil
}

// TODO: clean this up and move field strings into enum
func (m JsonFieldMapper) GetFieldValue(jIssue jira.Issue, fieldKey FieldKey) (interface{}, error) {
	j, err := jIssue.Fields.Unknowns.String(m.Config.GetFieldKey(GitHubData))

	if err != nil {
		return nil, err
	}

	var parsedJson map[string]interface{}

	json.Unmarshal([]byte(j), &parsedJson)

	switch fieldKey {
	case GitHubID:
		return parsedJson["githubId"], nil
	case GitHubNumber:
		return parsedJson["githubNumber"], nil
	case GitHubLabels:
		return parsedJson["githubLabels"], nil
	case GitHubStatus:
		return parsedJson["githubStatus"], nil
	case GitHubReporter:
		return parsedJson["githubReporter"], nil
	case LastISUpdate:
		return parsedJson["lastIssueSyncUpdate"], nil
	}
	return nil, err
}

func (m JsonFieldMapper) GetInt64FieldValue(issue *jira.Issue) int64 {
	id, _ := issue.Fields.Unknowns.Int(m.Config.GetFieldKey(GitHubID))
	return id
}

func (m JsonFieldMapper) MapFields(issue *github.Issue) jira.IssueFields {
	fields := jira.IssueFields{
		Type: jira.IssueType{
			Name: "Task", // TODO: Determine issue type
		},
		Project:     m.Config.GetProject(),
		Summary:     issue.GetTitle(),
		Description: issue.GetBody(),
		Unknowns:    map[string]interface{}{},
	}

	strs := make([]string, len(issue.Labels))
	for i, v := range issue.Labels {
		strs[i] = *v.Name
	}

	data := map[string]interface{}{
		"githubId": issue.GetID(),
		"githubNumber": issue.GetNumber(),
		"githubStatus":issue.GetState(),
		"githubReporter": issue.User.GetLogin(),
		"githubLabels": strs,
		"lastIssueSyncUpdate": time.Now().Format(DateFormat),
	}

	fields.Unknowns[m.Config.GetFieldKey(GitHubData)], _ = json.Marshal(data)

	return fields
}

func (m JsonFieldMapper) GetFieldIDs(client jira.Client) (map[FieldKey]string, error) {
	m.Config.log.Debug("Collecting field IDs.")
	req, err := client.NewRequest("GET", "/rest/api/2/field", nil)
	if err != nil {
		return map[FieldKey]string{}, err
	}
	jFields := new([]jiraField)

	_, err = client.Do(req, jFields)
	if err != nil {
		return map[FieldKey]string{}, err
	}

	fieldIDs := map[FieldKey]string{}

	for _, field := range *jFields {
		switch field.Name {
		case "GitHub Data":
			fieldIDs[GitHubData] = fmt.Sprint(field.Schema.CustomID)
		}
	}

	_, ok := fieldIDs[GitHubData]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'GitHub Data' custom field; check that it is named correctly")
	}

	m.Config.log.Debug("All fields have been checked.")

	return fieldIDs, nil
}