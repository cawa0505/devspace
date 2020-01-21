package docker

import (
	"testing"

	"github.com/devspace-cloud/devspace/pkg/util/log"
	"gotest.tools/assert"
)

// TODO: refactor helper package to make the docker-package testable
type buildImageTestCase struct {
	name string

	contextPath    string
	dockerfilePath string
	entrypoint     []string
	cmd            []string

	expectedErr string
}

func TestBuildImage(t *testing.T) {
	testCases := []buildImageTestCase{}

	for _, testCase := range testCases {
		builder := &Builder{}

		err := builder.BuildImage(testCase.contextPath, testCase.dockerfilePath, testCase.entrypoint, testCase.cmd, log.Discard)

		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error  in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error in testCase %s", testCase.name)
		}
	}
}
