// Package testdiscoveryservice is used for discover tests
package testdiscoveryservice

import (
	"context"
	"os/exec"

	"github.com/LambdaTest/synapse/pkg/core"
	"github.com/LambdaTest/synapse/pkg/global"
	"github.com/LambdaTest/synapse/pkg/logstream"
	"github.com/LambdaTest/synapse/pkg/lumber"
)

type testDiscoveryService struct {
	logger      lumber.Logger
	execManager core.ExecutionManager
}

// NewTestDiscoveryService creates and returns a new testDiscoveryService instance
func NewTestDiscoveryService(execManager core.ExecutionManager, logger lumber.Logger) core.TestDiscoveryService {
	tds := testDiscoveryService{logger: logger, execManager: execManager}
	return &tds
}

func (tds *testDiscoveryService) Discover(ctx context.Context,
	tasConfig *core.TASConfig,
	payload *core.Payload,
	secretData map[string]string,
	diff map[string]int) error {
	var target []string
	var envMap map[string]string
	if payload.EventType == core.EventPullRequest {
		target = tasConfig.Premerge.Patterns
		envMap = tasConfig.Premerge.EnvMap
	} else {
		target = tasConfig.Postmerge.Patterns
		envMap = tasConfig.Postmerge.EnvMap
	}
	tasYmlModified := false
	if _, ok := diff[payload.TasFileName]; ok {
		tasYmlModified = true
	}

	// discover all tests if tas.yml modified or if parent commit does not exists or smart run feature is set to false
	discoverAll := tasYmlModified || !payload.ParentCommitCoverageExists || !tasConfig.SmartRun

	args := []string{"--command", "discover"}
	if !discoverAll {
		for k, v := range diff {
			// in changed files we only have added or modified files.
			if v != core.FileRemoved {
				args = append(args, "--diff", k)
			}
		}
	}
	if tasConfig.ConfigFile != "" {
		args = append(args, "--config", tasConfig.ConfigFile)
	}

	for _, pattern := range target {
		args = append(args, "--pattern", pattern)
	}
	tds.logger.Debugf("Discovering tests at paths %+v", target)

	cmd := exec.CommandContext(ctx, global.FrameworkRunnerMap[tasConfig.Framework], args...)
	cmd.Dir = global.RepoDir
	envVars, err := tds.execManager.GetEnvVariables(envMap, secretData)
	if err != nil {
		tds.logger.Errorf("failed to parsed env variables, error: %v", err)
		return err
	}
	cmd.Env = envVars
	logWriter := lumber.NewWriter(tds.logger)
	defer logWriter.Close()
	maskWriter := logstream.NewMasker(logWriter, secretData)
	cmd.Stdout = maskWriter
	cmd.Stderr = maskWriter

	tds.logger.Debugf("Executing test discovery command: %s", cmd.String())
	if err := cmd.Run(); err != nil {
		tds.logger.Errorf("command %s of type %s failed with error: %v", cmd.String(), core.Discovery, err)
		return err
	}

	return nil
}