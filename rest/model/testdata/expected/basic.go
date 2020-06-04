// Code generated by rest/model/codegen.go. DO NOT EDIT.

package model

import "github.com/evergreen-ci/evergreen/model"

type Revision struct {
	Author          string `json:"author"`
	AuthorGithubUID int    `json:"author_github_u_i_d"`
}

func (m *APIRevision) BuildFromService(t model.Revision) error {
	m.Author = stringToString(t.Author)
	m.AuthorGithubUID = intToInt(t.AuthorGithubUID)
	return nil
}

func (m *APIRevision) ToService() (model.Revision, error) {
	out := model.Revision{}
	out.Author = stringToString(m.Author)
	out.AuthorGithubUID = intToInt(m.AuthorGithubUID)
	return out, nil
}
