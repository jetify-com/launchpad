package jetconfig

import "go.jetpack.io/launchpad/pkg/typeid"

const ProjectIDPrefix = "proj"

func NewProjectId() string {
	return typeid.New(ProjectIDPrefix).String()
}

func GetProjectSlug(projectID string) (string, error) {
	typeID := typeid.FromString(projectID)
	if typeID == nil {
		// projectID is in the wrong format
		return "", ErrInvalidProjectID
	}

	return typeID.Slug(), nil
}
