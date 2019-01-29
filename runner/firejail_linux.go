// +build linux

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/itchio/smaug/runner/policies"
	"github.com/pkg/errors"
)

type firejailRunner struct {
	params RunnerParams
}

var _ Runner = (*firejailRunner)(nil)

func newFirejailRunner(params RunnerParams) (Runner, error) {
	if params.FirejailParams.BinaryPath == "" {
		return nil, errors.Errorf("FirejailParams.BinaryPath must be set")
	}

	fr := &firejailRunner{
		params: params,
	}

	return fr, nil
}

func (fr *firejailRunner) Prepare() error {
	// nothing to prepare
	return nil
}

func (fr *firejailRunner) Run() error {
	params := fr.params
	consumer := params.Consumer

	firejailPath := params.FirejailParams.BinaryPath

	sandboxProfilePath := filepath.Join(params.InstallFolder, ".itch", "isolate-app.profile")
	consumer.Opf("Writing sandbox profile to (%s)", sandboxProfilePath)
	err := os.MkdirAll(filepath.Dir(sandboxProfilePath), 0755)
	if err != nil {
		return errors.WithStack(err)
	}
	
	sandboxTemplate, err := template.New("firejail-profile").Parse(policies.FirejailTemplate)
	if err != nil {
		return errors.WithStack(err)
	}
	
	sandboxFile, err := os.OpenFile(sandboxProfilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return errors.WithStack(err)
	}
	
	err = sandboxTemplate.Execute(sandboxFile, params)
	if err != nil {
		return errors.WithStack(err)
	}

	consumer.Opf("Running (%s) through firejail", params.FullTargetPath)

	var args []string
	args = append(args, fmt.Sprintf("--profile=%s", sandboxProfilePath))
	args = append(args, "--")
	args = append(args, params.FullTargetPath)
	args = append(args, params.Args...)

	cmd := exec.Command(firejailPath, args...)
	cmd.Dir = params.Dir
	cmd.Env = params.Env
	cmd.Stdout = params.Stdout
	cmd.Stderr = params.Stderr

	pg, err := NewProcessGroup(consumer, cmd, params.Ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	err = pg.AfterStart()
	if err != nil {
		return errors.WithStack(err)
	}

	err = cmd.Start()
	if err != nil {
		return errors.WithStack(err)
	}

	err = pg.Wait()
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
