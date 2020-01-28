package build

import (
	"time"

	"github.com/devspace-cloud/devspace/e2e/utils"
	"github.com/devspace-cloud/devspace/pkg/devspace/build"
	"github.com/devspace-cloud/devspace/pkg/devspace/config/generated"
	"github.com/devspace-cloud/devspace/pkg/devspace/config/versions/latest"
	"github.com/devspace-cloud/devspace/pkg/devspace/kubectl"
	"github.com/devspace-cloud/devspace/pkg/util/log"
	"github.com/pkg/errors"
)

type customFactory struct {
	*utils.BaseCustomFactory
	ctrl        build.Controller
	builtImages map[string]string
}

// NewBuildController implements interface
func (c *customFactory) NewBuildController(config *latest.Config, cache *generated.CacheConfig, client kubectl.Client) build.Controller {
	c.ctrl = build.NewController(config, cache, client)
	return c
}
func (c *customFactory) Build(options *build.Options, log log.Logger) (map[string]string, error) {
	m, err := c.ctrl.Build(options, log)
	c.builtImages = m

	return m, err
}

type Runner struct{}

var RunNew = &Runner{}

func (r *Runner) SubTests() []string {
	subTests := []string{}
	for k := range availableSubTests {
		subTests = append(subTests, k)
	}

	return subTests
}

var availableSubTests = map[string]func(factory *customFactory, logger log.Logger) error{
	"default": runDefault,
}

func (r *Runner) Run(subTests []string, ns string, pwd string, logger log.Logger, verbose bool, timeout int) error {
	logger.Info("Run test 'build'")

	// Populates the tests to run with all the available sub tests if no sub tests are specified
	if len(subTests) == 0 {
		for subTestName := range availableSubTests {
			subTests = append(subTests, subTestName)
		}
	}

	f := &customFactory{
		BaseCustomFactory: &utils.BaseCustomFactory{
			Pwd:     pwd,
			Verbose: verbose,
			Timeout: timeout,
		},
	}

	// Runs the tests
	for _, subTestName := range subTests {
		f.ResetLog()
		c1 := make(chan error, 1)

		go func() {
			err := func() error {
				// f.Namespace = utils.GenerateNamespaceName("test-build-" + subTestName)

				err := availableSubTests[subTestName](f, logger)
				utils.PrintTestResult("build", subTestName, err, logger)
				if err != nil {
					return errors.Errorf("test 'build' failed: %s %v", f.GetLogContents(), err)
				}

				return nil
			}()
			c1 <- err
		}()

		select {
		case err := <-c1:
			if err != nil {
				return err
			}
		case <-time.After(time.Duration(timeout) * time.Second):
			return errors.Errorf("Timeout error - the test did not return within the specified timeout of %v seconds: %s", timeout, f.GetLogContents())
		}
	}

	return nil
}

func beforeTest(f *customFactory, testFolder string) error {
	dirPath, _, err := utils.CreateTempDir()
	if err != nil {
		return err
	}

	err = utils.Copy(f.Pwd+"/tests/build/testdata/"+testFolder, dirPath)
	if err != nil {
		return err
	}

	err = utils.ChangeWorkingDir(dirPath, f.GetLog())
	if err != nil {
		return err
	}

	return nil
}

func afterTest(f *customFactory) {
	utils.DeleteTempAndResetWorkingDir(f.DirPath, f.Pwd, f.GetLog())
}
