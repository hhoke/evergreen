package command

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/evergreen-ci/evergreen/apimodels"
	"github.com/evergreen-ci/evergreen/model"
	"github.com/evergreen-ci/evergreen/model/task"
	"github.com/evergreen-ci/evergreen/rest/client"
	"github.com/evergreen-ci/evergreen/util"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3PullParseParams(t *testing.T) {
	for testName, testCase := range map[string]func(*testing.T, *s3Pull){
		"SetsValues": func(t *testing.T, c *s3Pull) {
			params := map[string]interface{}{
				"exclude":           "exclude_pattern",
				"build_variants":    []string{"some_build_variant"},
				"max_retries":       uint(5),
				"task":              "task_name",
				"working_directory": "working_dir",
				"delete_on_sync":    true,
			}
			require.NoError(t, c.ParseParams(params))
			assert.Equal(t, params["exclude"], c.ExcludeFilter)
			assert.Equal(t, params["build_variants"], c.BuildVariants)
			assert.Equal(t, params["max_retries"], c.MaxRetries)
			assert.Equal(t, params["task"], c.TaskName)
			assert.Equal(t, params["working_directory"], c.WorkingDir)
			assert.Equal(t, params["delete_on_sync"], c.DeleteOnSync)
		},
		"RequiresWorkingDirectory": func(t *testing.T, c *s3Pull) {
			assert.Error(t, c.ParseParams(map[string]interface{}{}))
		},
	} {
		t.Run(testName, func(t *testing.T) {
			c := &s3Pull{}
			testCase(t, c)
		})
	}
}

func TestS3PullExecute(t *testing.T) {
	taskName := "test"

	for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, c *s3Pull, comm *client.Mock, logger client.LoggerProducer, conf *model.TaskConfig, bucketDir string){
		"PullsTaskDirectoryFromS3": func(ctx context.Context, t *testing.T, c *s3Pull, comm *client.Mock, logger client.LoggerProducer, conf *model.TaskConfig, bucketDir string) {
			taskDir := filepath.Join(bucketDir, conf.S3Path(taskName))
			require.NoError(t, os.MkdirAll(taskDir, 0777))
			tmpFile, err := ioutil.TempFile(taskDir, "s3-pull-file")
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, os.RemoveAll(tmpFile.Name()))
			}()
			fileContent := []byte("foobar")
			_, err = tmpFile.Write(fileContent)
			assert.NoError(t, tmpFile.Close())
			require.NoError(t, err)

			c.WorkingDir, err = ioutil.TempDir("", "s3-pull-output")
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, os.RemoveAll(c.WorkingDir))
			}()

			require.NoError(t, c.Execute(ctx, comm, logger, conf))
			pulledContent, err := ioutil.ReadFile(filepath.Join(c.WorkingDir, filepath.Base(tmpFile.Name())))
			require.NoError(t, err)
			assert.Equal(t, pulledContent, fileContent)
		},
		"IgnoresFilesExcludedByFilter": func(ctx context.Context, t *testing.T, c *s3Pull, comm *client.Mock, logger client.LoggerProducer, conf *model.TaskConfig, bucketDir string) {
			taskDir := filepath.Join(bucketDir, conf.S3Path(taskName))
			require.NoError(t, os.MkdirAll(taskDir, 0777))
			tmpFile, err := ioutil.TempFile(taskDir, "s3-pull-file")
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, os.RemoveAll(tmpFile.Name()))
			}()
			_, err = tmpFile.Write([]byte("foobar"))
			assert.NoError(t, tmpFile.Close())
			require.NoError(t, err)

			c.WorkingDir, err = ioutil.TempDir("", "s3-pull-output")
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, os.RemoveAll(c.WorkingDir))
			}()

			c.ExcludeFilter = ".*"
			require.NoError(t, c.Execute(ctx, comm, logger, conf))

			files, err := ioutil.ReadDir(c.WorkingDir)
			require.NoError(t, err)
			assert.Empty(t, files)
		},
		"NoopsIfIgnoringBuildVariant": func(ctx context.Context, t *testing.T, c *s3Pull, comm *client.Mock, logger client.LoggerProducer, conf *model.TaskConfig, bucketDir string) {
			taskDir := filepath.Join(bucketDir, conf.S3Path(taskName))
			require.NoError(t, os.MkdirAll(taskDir, 0777))
			tmpFile, err := ioutil.TempFile(taskDir, "s3-pull-file")
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, os.RemoveAll(tmpFile.Name()))
			}()
			fileContent := []byte("foobar")
			_, err = tmpFile.Write(fileContent)
			assert.NoError(t, tmpFile.Close())
			require.NoError(t, err)

			c.WorkingDir, err = ioutil.TempDir("", "s3-pull-output")
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, os.RemoveAll(c.WorkingDir))
			}()

			c.BuildVariants = []string{"other_build_variant"}
			require.NoError(t, c.Execute(ctx, comm, logger, conf))

			files, err := ioutil.ReadDir(c.WorkingDir)
			require.NoError(t, err)
			assert.Empty(t, files)
		},
		"ExpandsParameters": func(ctx context.Context, t *testing.T, c *s3Pull, comm *client.Mock, logger client.LoggerProducer, conf *model.TaskConfig, bucketDir string) {
			tmpDir, err := ioutil.TempDir("", "s3-pull")
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, os.RemoveAll(tmpDir))
			}()

			c.WorkingDir, err = ioutil.TempDir("", "s3-pull-output")
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, os.RemoveAll(c.WorkingDir))
			}()

			c.ExcludeFilter = "${exclude_filter}"
			excludeFilterExpansion := "expanded_exclude_filter"
			conf.Expansions = util.NewExpansions(map[string]string{
				"exclude_filter": excludeFilterExpansion,
			})
			assert.NoError(t, c.Execute(ctx, comm, logger, conf))
			assert.Equal(t, excludeFilterExpansion, c.ExcludeFilter)
		},
		"FailsWithoutS3Key": func(ctx context.Context, t *testing.T, c *s3Pull, comm *client.Mock, logger client.LoggerProducer, conf *model.TaskConfig, bucketDir string) {
			c.bucket = nil
			conf.S3Data.Key = ""
			assert.Error(t, c.Execute(ctx, comm, logger, conf))
		},
		"FailsWithoutS3Secret": func(ctx context.Context, t *testing.T, c *s3Pull, comm *client.Mock, logger client.LoggerProducer, conf *model.TaskConfig, bucketDir string) {
			c.bucket = nil
			conf.S3Data.Secret = ""
			assert.Error(t, c.Execute(ctx, comm, logger, conf))
		},
		"FailsWithoutS3BucketName": func(ctx context.Context, t *testing.T, c *s3Pull, comm *client.Mock, logger client.LoggerProducer, conf *model.TaskConfig, bucketDir string) {
			c.bucket = nil
			conf.S3Data.Bucket = ""
			assert.Error(t, c.Execute(ctx, comm, logger, conf))
		},
	} {
		t.Run(testName, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			conf := &model.TaskConfig{
				Task: &task.Task{
					Id:           "id",
					Secret:       "secret",
					Version:      "version",
					BuildVariant: "build_variant",
					DisplayName:  "display_name",
				},
				BuildVariant: &model.BuildVariant{
					Name: "build_variant",
				},
				ProjectRef: &model.ProjectRef{
					Identifier: "project_identifier",
				},
				S3Data: apimodels.S3TaskSetupData{
					Key:    "s3_key",
					Secret: "s3_secret",
					Bucket: "s3_bucket",
				},
			}
			comm := client.NewMock("localhost")
			logger, err := comm.GetLoggerProducer(ctx, client.TaskData{
				ID:     conf.Task.Id,
				Secret: conf.Task.Secret,
			}, nil)
			require.NoError(t, err)
			tmpDir, err := ioutil.TempDir("", "s3-pull-bucket")
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, os.RemoveAll(tmpDir))
			}()
			c := &s3Pull{TaskName: taskName}
			c.bucket, err = pail.NewLocalBucket(pail.LocalOptions{
				Path: tmpDir,
			})
			require.NoError(t, err)
			testCase(ctx, t, c, comm, logger, conf, tmpDir)
		})
	}
}