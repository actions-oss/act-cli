package tart

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/actions-oss/act-cli/pkg/common"
	"github.com/actions-oss/act-cli/pkg/container"
)

type Environment struct {
	container.HostEnvironment
	vm       *VM
	Config   Config
	Env      *Env
	Miscpath string
}

// "/Volumes/My Shared Files/act/"
func (e *Environment) ToHostPath(path string) string {
	actPath := filepath.Clean("/private/tmp/act/")
	altPath := filepath.Clean(path)
	if strings.HasPrefix(altPath, actPath) {
		return e.Miscpath + altPath[len(actPath):]
	}
	return altPath
}

func (e *Environment) ToContainerPath(path string) string {
	path = e.HostEnvironment.ToContainerPath(path)
	actPath := filepath.Clean(e.Miscpath)
	altPath := filepath.Clean(path)
	if strings.HasPrefix(altPath, actPath) {
		return "/private/tmp/act/" + altPath[len(actPath):]
	}
	return altPath
}

func (e *Environment) Exec(command []string /*cmdline string, */, env map[string]string, user, workdir string) common.Executor {
	return e.ExecWithCmdLine(command, "", env, user, workdir)
}

func (e *Environment) ExecWithCmdLine(command []string, cmdline string, env map[string]string, user, workdir string) common.Executor {
	return func(ctx context.Context) error {
		if err := e.exec(ctx, command, cmdline, env, user, workdir); err != nil {
			select {
			case <-ctx.Done():
				return fmt.Errorf("this step has been cancelled: %w", err)
			default:
				return err
			}
		}
		return nil
	}
}

func (e *Environment) Start(b bool) common.Executor {
	return e.HostEnvironment.Start(b).Then(func(ctx context.Context) error {
		return e.start(ctx)
	})
}

func (e *Environment) start(ctx context.Context) error {
	actEnv := e.Env

	config := e.Config

	if config.AlwaysPull {
		log.Printf("Pulling the latest version of %s...\n", actEnv.JobImage)
		_, _, err := ExecWithEnv(ctx, nil,
			"pull", actEnv.JobImage)
		if err != nil {
			return err
		}
	}

	log.Println("Cloning and configuring a new VM...")
	vm, err := CreateNewVM(ctx, *actEnv, 0, 0)
	if err != nil {
		_ = e.Stop(ctx)
		return err
	}
	var customDirectoryMounts []string
	_ = os.MkdirAll(e.Miscpath, 0666)
	customDirectoryMounts = append(customDirectoryMounts, "act:"+e.Miscpath)
	e.vm = vm
	err = vm.Start(config, actEnv, customDirectoryMounts)
	if err != nil {
		_ = e.Stop(ctx)
		return err
	}

	return e.execRaw(ctx, "ln -sf '/Volumes/My Shared Files/act' /private/tmp/act")
}
func (e *Environment) Stop(ctx context.Context) error {
	log.Println("Stop VM?")

	actEnv := e.Env

	var vm *VM
	if e.vm != nil {
		vm = e.vm
	} else {
		vm = ExistingVM(*actEnv)
	}

	if err := vm.Stop(); err != nil {
		log.Printf("Failed to stop VM: %v", err)
	}

	if err := vm.Delete(); err != nil {
		log.Printf("Failed to delete VM: %v", err)

		return err
	}

	return nil
}

func (e *Environment) Remove() common.Executor {
	return func(ctx context.Context) error {
		_ = e.Stop(ctx)
		log.Println("Remove VM?")
		if e.CleanUp != nil {
			e.CleanUp()
		}
		_ = os.RemoveAll(e.Path)
		return e.Close()(ctx)
	}
}
func (e *Environment) exec(ctx context.Context, command []string, _ string, env map[string]string, _, workdir string) error {
	var wd string
	if workdir != "" {
		if filepath.IsAbs(workdir) {
			wd = filepath.Clean(workdir)
		} else {
			wd = filepath.Clean(filepath.Join(e.Path, workdir))
		}
	} else {
		wd = e.ToContainerPath(e.Path)
	}
	envs := ""
	for k, v := range env {
		envs += shellquote.Join(k) + "=" + shellquote.Join(v) + " "
	}
	return e.execRaw(ctx, "cd "+shellquote.Join(wd)+"\nenv "+envs+shellquote.Join(command...)+"\nexit $?")
}

func (e *Environment) execRaw(ctx context.Context, script string) error {
	actEnv := e.Env

	var vm *VM
	if e.vm != nil {
		vm = e.vm
	} else {
		vm = ExistingVM(*actEnv)
	}

	// Monitor "tart run" command's output so it's not silenced
	go vm.MonitorTartRunOutput()

	config := e.Config

	ssh, err := vm.OpenSSH(ctx, config)
	if err != nil {
		return err
	}
	defer ssh.Close()

	session, err := ssh.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	os.Stdout.WriteString(script + "\n")

	session.Stdin = strings.NewReader(
		script,
	)
	session.Stdout = e.StdOut
	session.Stderr = e.StdOut

	err = session.Shell()
	if err != nil {
		return err
	}

	return session.Wait()
}

func (e *Environment) GetActPath() string {
	return e.ToContainerPath(e.HostEnvironment.GetActPath())
}

func (e *Environment) Copy(destPath string, files ...*container.FileEntry) common.Executor {
	return e.HostEnvironment.Copy(e.ToHostPath(destPath), files...)
}
func (e *Environment) CopyTarStream(ctx context.Context, destPath string, tarStream io.Reader) error {
	return e.HostEnvironment.CopyTarStream(ctx, e.ToHostPath(destPath), tarStream)
}
func (e *Environment) CopyDir(destPath string, srcPath string, useGitIgnore bool) common.Executor {
	return e.HostEnvironment.CopyDir(e.ToHostPath(destPath), srcPath, useGitIgnore)
}
func (e *Environment) GetContainerArchive(ctx context.Context, srcPath string) (io.ReadCloser, error) {
	return e.HostEnvironment.GetContainerArchive(ctx, e.ToHostPath(srcPath))
}

func (e *Environment) GetRunnerContext(ctx context.Context) map[string]interface{} {
	rctx := e.HostEnvironment.GetRunnerContext(ctx)
	rctx["temp"] = e.ToContainerPath(e.TmpDir)
	rctx["tool_cache"] = e.ToContainerPath(e.ToolCache)
	return rctx
}
