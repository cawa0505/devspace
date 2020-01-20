package dependency

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/devspace-cloud/devspace/pkg/devspace/config/generated"
	fakegeneratedloader "github.com/devspace-cloud/devspace/pkg/devspace/config/generated/testing"
	"github.com/devspace-cloud/devspace/pkg/devspace/config/loader"
	"github.com/devspace-cloud/devspace/pkg/devspace/config/versions/latest"
	fakedeploy "github.com/devspace-cloud/devspace/pkg/devspace/deploy/testing"
	"github.com/devspace-cloud/devspace/pkg/util/fsutil"
	"github.com/devspace-cloud/devspace/pkg/util/hash"
	"github.com/devspace-cloud/devspace/pkg/util/log"

	"gotest.tools/assert"
)

type fakeResolver struct {
	resolvedDependencies []*Dependency
}

var replaceWithHash = "replaceThisWithHash"

func (r *fakeResolver) Resolve(update bool) ([]*Dependency, error) {
	for _, dep := range r.resolvedDependencies {
		directoryHash, _ := hash.DirectoryExcludes(dep.LocalPath, []string{".git", ".devspace"}, true)
		for _, profile := range dep.DependencyCache.Profiles {
			for key, val := range profile.Dependencies {
				if val == replaceWithHash {
					profile.Dependencies[key] = directoryHash
				}
			}
		}
		dep.deployController = &fakedeploy.FakeController{}
		dep.generatedSaver = &fakegeneratedloader.Loader{}
	}
	return r.resolvedDependencies, nil
}

type updateAllTestCase struct {
	name string

	files            map[string]string
	dependencyTasks  []*latest.DependencyConfig
	activeConfig     *generated.CacheConfig
	allowCyclicParam bool

	expectedErr string
}

func TestUpdateAll(t *testing.T) {
	testCases := []updateAllTestCase{
		updateAllTestCase{
			name: "No Dependencies to update",
		},
		updateAllTestCase{
			name: "Update one dependency",
			files: map[string]string{
				"devspace.yaml":         "version: v1beta3",
				"someDir/devspace.yaml": "version: v1beta3",
			},
			dependencyTasks: []*latest.DependencyConfig{
				&latest.DependencyConfig{
					Source: &latest.SourceConfig{
						Path: "someDir",
					},
				},
			},
			activeConfig: &generated.CacheConfig{
				Images: map[string]*generated.ImageCache{
					"default": &generated.ImageCache{
						Tag: "1.15", // This will be appended to nginx during deploy
					},
				},
				Dependencies: map[string]string{},
			},
			allowCyclicParam: true,
		},
	}

	dir, err := ioutil.TempDir("", "testFolder")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}

	wdBackup, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Error changing working directory: %v", err)
	}

	// Delete temp folder
	defer func() {
		err = os.Chdir(wdBackup)
		if err != nil {
			t.Fatalf("Error changing dir back: %v", err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("Error removing dir: %v", err)
		}
	}()

	for _, testCase := range testCases {
		for path, content := range testCase.files {
			err = fsutil.WriteToFile([]byte(content), path)
			assert.NilError(t, err, "Error writing file in testCase %s", testCase.name)
		}

		testConfig := &latest.Config{
			Dependencies: testCase.dependencyTasks,
		}
		generatedConfig := &generated.Config{
			ActiveProfile: "default",
			Profiles: map[string]*generated.CacheConfig{
				"default": testCase.activeConfig,
			},
		}

		manager, err := NewManager(testConfig, generatedConfig, nil, testCase.allowCyclicParam, &loader.ConfigOptions{}, log.Discard)
		assert.NilError(t, err, "Error creating manager in testCase %s", testCase.name)

		err = manager.UpdateAll()

		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error updating all in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error from UpdateALl in testCase %s", testCase.name)
		}

		for path := range testCase.files {
			err = os.Remove(path)
			assert.NilError(t, err, "Error removing file in testCase %s", testCase.name)
		}
	}
}

type buildAllTestCase struct {
	name string

	files                map[string]string
	dependencyTasks      []*latest.DependencyConfig
	resolvedDependencies []*Dependency
	options              BuildOptions

	expectedErr string
}

func TestBuildAll(t *testing.T) {
	dir, err := ioutil.TempDir("", "testFolder")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}

	wdBackup, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Error changing working directory: %v", err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Delete temp folder
	defer func() {
		err = os.Chdir(wdBackup)
		if err != nil {
			t.Fatalf("Error changing dir back: %v", err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("Error removing dir: %v", err)
		}
	}()

	testCases := []buildAllTestCase{
		buildAllTestCase{
			name: "No Dependencies to build",
		},
		buildAllTestCase{
			name:  "Build one dependency",
			files: map[string]string{},
			dependencyTasks: []*latest.DependencyConfig{
				&latest.DependencyConfig{},
			},
			resolvedDependencies: []*Dependency{
				&Dependency{
					LocalPath: "./",
					DependencyCache: &generated.Config{
						ActiveProfile: "",
						Profiles: map[string]*generated.CacheConfig{
							"": &generated.CacheConfig{
								Dependencies: map[string]string{
									"": replaceWithHash,
								},
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		for path, content := range testCase.files {
			err = fsutil.WriteToFile([]byte(content), path)
			assert.NilError(t, err, "Error writing file in testCase %s", testCase.name)
		}

		manager := &manager{
			config: &latest.Config{
				Dependencies: testCase.dependencyTasks,
			},
			log: log.Discard,
			resolver: &fakeResolver{
				resolvedDependencies: testCase.resolvedDependencies,
			},
		}

		err = manager.BuildAll(testCase.options)

		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error deploying all in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error from DeployALl in testCase %s", testCase.name)
		}

		for path := range testCase.files {
			err = os.Remove(path)
			assert.NilError(t, err, "Error removing file in testCase %s", testCase.name)
		}
	}
}

type deployAllTestCase struct {
	name string

	files                map[string]string
	dependencyTasks      []*latest.DependencyConfig
	resolvedDependencies []*Dependency
	options              DeployOptions

	expectedErr string
}

func TestDeployAll(t *testing.T) {
	dir, err := ioutil.TempDir("", "testFolder")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}

	wdBackup, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Error changing working directory: %v", err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Delete temp folder
	defer func() {
		err = os.Chdir(wdBackup)
		if err != nil {
			t.Fatalf("Error changing dir back: %v", err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("Error removing dir: %v", err)
		}
	}()

	testCases := []deployAllTestCase{
		deployAllTestCase{
			name: "No Dependencies to deploy",
		},
		deployAllTestCase{
			name:  "Deploy one dependency",
			files: map[string]string{},
			dependencyTasks: []*latest.DependencyConfig{
				&latest.DependencyConfig{},
			},
			resolvedDependencies: []*Dependency{
				&Dependency{
					LocalPath: "./",
					DependencyCache: &generated.Config{
						ActiveProfile: "",
						Profiles: map[string]*generated.CacheConfig{
							"": &generated.CacheConfig{
								Dependencies: map[string]string{
									"": replaceWithHash,
								},
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		for path, content := range testCase.files {
			err = fsutil.WriteToFile([]byte(content), path)
			assert.NilError(t, err, "Error writing file in testCase %s", testCase.name)
		}

		manager := &manager{
			config: &latest.Config{
				Dependencies: testCase.dependencyTasks,
			},
			log: log.Discard,
			resolver: &fakeResolver{
				resolvedDependencies: testCase.resolvedDependencies,
			},
		}

		err = manager.DeployAll(testCase.options)

		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error deploying all in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error from DeployALl in testCase %s", testCase.name)
		}

		for path := range testCase.files {
			err = os.Remove(path)
			assert.NilError(t, err, "Error removing file in testCase %s", testCase.name)
		}
	}
}

type purgeAllTestCase struct {
	name string

	files                map[string]string
	dependencyTasks      []*latest.DependencyConfig
	resolvedDependencies []*Dependency
	verboseParam         bool

	expectedErr string
}

func TestPurgeAll(t *testing.T) {
	dir, err := ioutil.TempDir("", "testFolder")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	wdBackup, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Error changing working directory: %v", err)
	}

	// Delete temp folder
	defer func() {
		err = os.Chdir(wdBackup)
		if err != nil {
			t.Fatalf("Error changing dir back: %v", err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("Error removing dir: %v", err)
		}
	}()

	testCases := []purgeAllTestCase{
		purgeAllTestCase{
			name: "No Dependencies to update",
		},
		purgeAllTestCase{
			name: "Update one dependency",
			files: map[string]string{
				"devspace.yaml":         "",
				"someDir/devspace.yaml": "",
			},
			dependencyTasks: []*latest.DependencyConfig{
				&latest.DependencyConfig{},
			},
			resolvedDependencies: []*Dependency{
				&Dependency{
					LocalPath: "./",
					DependencyCache: &generated.Config{
						ActiveProfile: "",
						Profiles: map[string]*generated.CacheConfig{
							"": &generated.CacheConfig{
								Dependencies: map[string]string{
									"": replaceWithHash,
								},
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		for path, content := range testCase.files {
			err = fsutil.WriteToFile([]byte(content), path)
			assert.NilError(t, err, "Error writing file in testCase %s", testCase.name)
		}

		manager := &manager{
			config: &latest.Config{
				Dependencies: testCase.dependencyTasks,
			},
			log: log.Discard,
			resolver: &fakeResolver{
				resolvedDependencies: testCase.resolvedDependencies,
			},
		}

		err = manager.PurgeAll(testCase.verboseParam)

		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error purging all in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error from PurgeALl in testCase %s", testCase.name)
		}

		for path := range testCase.files {
			err = os.Remove(path)
			assert.NilError(t, err, "Error removing file in testCase %s", testCase.name)
		}
	}
}
