package launchpad

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGcpImageRepoValidation(t *testing.T) {
	var cases = []struct {
		testName  string
		imageName string
		expected  bool
	}{
		{
			"imageRepoCanonicalFormat",
			"us-central1-docker.pkg.dev/jetpack-dev/savil-cluster-test-2/py-dockerfile",
			true,
		},
		{
			"imageRepoMissingName",
			"us-central1-docker.pkg.dev/jetpack-dev/savil-cluster-test-2",
			false,
		},
		{
			"imageRepoMissingNameWithTrailingSlash",
			"us-central1-docker.pkg.dev/jetpack-dev/savil-cluster-test-2/",
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.testName, func(t *testing.T) {
			assert := assert.New(t)
			err := validatePublishPlan(&PublishPlan{
				images: []*PublishImagePlan{{
					remoteImageName: tc.imageName,
				}},
				registry: &ImageRegistry{
					host: gcpRegistryHost,
				},
			})
			if tc.expected {
				assert.NoError(err)
			} else {
				assert.Error(err)
			}
		})
	}
}
