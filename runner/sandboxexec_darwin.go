//go:build darwin

package runner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/itchio/ox/macox"
	"github.com/itchio/smaug/runner/policies"
)

var investigateSandbox = os.Getenv("INVESTIGATE_SANDBOX") == "1"
var sandboxExecLookPath = exec.LookPath
var sandboxExecCommand = exec.Command
var newSimpleRunnerForSandboxExec = newSimpleRunner
var runAppBundleForSandboxExec = RunAppBundle

type sandboxExecPolicyMode string

const (
	sandboxExecPolicyModeBalanced sandboxExecPolicyMode = "balanced"
	sandboxExecPolicyModeLegacy   sandboxExecPolicyMode = "legacy"
)

type sandboxExecPolicyData struct {
	UserLibrary         string
	InstallLocation     string
	AllowNetwork        bool
	LegacyCompatibility bool
}

type sandboxExecRunner struct {
	params          RunnerParams
	target          *MacLaunchTarget
	sandboxExecPath string
	policyMode      sandboxExecPolicyMode
}

var _ Runner = (*sandboxExecRunner)(nil)

func newSandboxExecRunner(params RunnerParams) (Runner, error) {
	target, err := PrepareMacLaunchTarget(params)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	params.FullTargetPath = target.Path

	ser := &sandboxExecRunner{
		params: params,
		target: target,
	}

	return ser, nil
}

func (ser *sandboxExecRunner) Prepare() error {
	consumer := ser.params.Consumer

	sandboxExecPath, err := sandboxExecLookPath("sandbox-exec")
	if err != nil {
		consumer.Warnf("While resolving sandbox-exec path: %s", err.Error())
		return errors.New("Cannot set up itch.io sandbox, see logs for details")
	}
	ser.sandboxExecPath = sandboxExecPath

	// make sure sandbox-exec is runnable
	cmd := sandboxExecCommand(ser.sandboxExecPath, "-n", "no-network", "true")
	err = cmd.Run()
	if err != nil {
		consumer.Warnf("While verifying sandbox-exec: %s", err.Error())
		return errors.New("Cannot set up itch.io sandbox, see logs for details")
	}

	return nil
}

func (ser *sandboxExecRunner) SandboxProfilePath() string {
	params := ser.params
	return filepath.Join(params.InstallFolder, ".itch", "isolate-app.sb")
}

func escapeSBPLString(input string) string {
	escaped := strings.ReplaceAll(input, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	return escaped
}

func sandboxExecPolicyModeFromConfig(value SandboxPolicyMode) (sandboxExecPolicyMode, string, bool) {
	raw := strings.ToLower(strings.TrimSpace(string(value)))
	switch raw {
	case "", string(sandboxExecPolicyModeBalanced):
		return sandboxExecPolicyModeBalanced, raw, true
	case string(sandboxExecPolicyModeLegacy):
		return sandboxExecPolicyModeLegacy, raw, true
	default:
		return sandboxExecPolicyModeBalanced, raw, false
	}
}

func (ser *sandboxExecRunner) WriteSandboxProfile() error {
	sandboxProfilePath := ser.SandboxProfilePath()

	params := ser.params
	consumer := params.Consumer
	consumer.Opf("Writing sandbox profile to (%s)", sandboxProfilePath)
	err := os.MkdirAll(filepath.Dir(sandboxProfilePath), 0755)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	userLibrary, err := macox.GetLibraryPath()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	mode, rawMode, validMode := sandboxExecPolicyModeFromConfig(params.SandboxConfig.PolicyMode)
	if !validMode {
		consumer.Warnf("Unknown SandboxConfig.PolicyMode value (%s), defaulting to (%s)", rawMode, sandboxExecPolicyModeBalanced)
	}
	ser.policyMode = mode

	sandboxTemplate, err := template.New("sandboxexec-profile").Parse(policies.SandboxExecTemplate)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	sandboxData := sandboxExecPolicyData{
		UserLibrary:         escapeSBPLString(userLibrary),
		InstallLocation:     escapeSBPLString(params.InstallFolder),
		AllowNetwork:        !params.SandboxConfig.NoNetwork,
		LegacyCompatibility: mode == sandboxExecPolicyModeLegacy,
	}

	var sandboxBuf bytes.Buffer
	err = sandboxTemplate.Execute(&sandboxBuf, sandboxData)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = os.WriteFile(sandboxProfilePath, sandboxBuf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	consumer.Infof("Using sandbox-exec policy mode (%s)", mode)
	return nil
}

func (ser *sandboxExecRunner) logSandboxFailure(err error) {
	consumer := ser.params.Consumer
	consumer.Warnf("Sandboxed launch failed in (%s) mode: %s", ser.policyMode, err.Error())
	consumer.Warnf("Sandbox profile path: (%s)", ser.SandboxProfilePath())
	if ser.policyMode != sandboxExecPolicyModeLegacy {
		consumer.Warnf("For compatibility debugging, set SandboxConfig.PolicyMode to (%s)", sandboxExecPolicyModeLegacy)
	}
}

func (ser *sandboxExecRunner) Run() error {
	params := ser.params
	consumer := params.Consumer

	if ser.sandboxExecPath == "" {
		err := ser.Prepare()
		if err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	err := ser.WriteSandboxProfile()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	sandboxEnv := collectAllowedEnvDarwin(params.Env, os.Environ(), params.SandboxConfig.AllowEnv)

	if !ser.target.IsAppBundle {
		consumer.Infof("Dealing with naked executable, launching via sandbox-exec directly")
		targetPath := params.FullTargetPath
		args := []string{
			"-f",
			ser.SandboxProfilePath(),
			targetPath,
		}
		args = append(args, params.Args...)

		simpleParams := params
		simpleParams.FullTargetPath = ser.sandboxExecPath
		simpleParams.Args = args
		simpleParams.Env = sandboxEnv
		simpleRunner, err := newSimpleRunnerForSandboxExec(simpleParams)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		err = simpleRunner.Prepare()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		err = simpleRunner.Run()
		if err != nil {
			ser.logSandboxFailure(err)
			return fmt.Errorf("%w", err)
		}
		return nil
	}

	consumer.Infof("Creating shim app bundle to enable sandboxing")
	realBundlePath := params.FullTargetPath

	binaryPath, err := macox.GetExecutablePath(realBundlePath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	binaryName := filepath.Base(binaryPath)

	workDir, err := os.MkdirTemp("", "butler-shim-bundle")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer os.RemoveAll(workDir)

	shimBundlePath := filepath.Join(
		workDir,
		filepath.Base(realBundlePath),
	)
	consumer.Opf("Generating shim bundle as (%s)", shimBundlePath)

	shimBinaryPath := filepath.Join(
		shimBundlePath,
		"Contents",
		"MacOS",
		binaryName,
	)
	err = os.MkdirAll(filepath.Dir(shimBinaryPath), 0755)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	var shimLines []string
	shimLines = append(shimLines, "#!/bin/sh")
	shimLines = append(shimLines, "set -e")
	if params.Dir != "" {
		shimLines = append(shimLines, fmt.Sprintf("cd %q", params.Dir))
	}
	shimLines = append(
		shimLines,
		fmt.Sprintf("exec %q -f %q %q \"$@\"", ser.sandboxExecPath, ser.SandboxProfilePath(), binaryPath),
	)
	shimBinaryContents := strings.Join(shimLines, "\n") + "\n"

	err = os.WriteFile(shimBinaryPath, []byte(shimBinaryContents), 0744)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = os.Symlink(
		filepath.Join(realBundlePath, "Contents", "Resources"),
		filepath.Join(shimBundlePath, "Contents", "Resources"),
	)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = os.Symlink(
		filepath.Join(realBundlePath, "Contents", "Info.plist"),
		filepath.Join(shimBundlePath, "Contents", "Info.plist"),
	)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if investigateSandbox {
		consumer.Warnf("Wrote shim app to (%s), waiting forever because INVESTIGATE_SANDBOX is set to 1", shimBundlePath)
		for {
			time.Sleep(1 * time.Second)
		}
	}

	consumer.Statf("All set, hope for the best")

	sandboxParams := params
	sandboxParams.Env = sandboxEnv

	err = runAppBundleForSandboxExec(
		sandboxParams,
		shimBundlePath,
	)
	if err != nil {
		ser.logSandboxFailure(err)
		return fmt.Errorf("%w", err)
	}
	return nil
}
