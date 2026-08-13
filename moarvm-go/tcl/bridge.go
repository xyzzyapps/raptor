package tcl

import (
	"context"
	"fmt"
	"moarvm-go/engine"
	"strings"
)


// Bridge binds a MoarVM engine instance into a Tcl interpreter under the moar:: namespace.
type Bridge struct {
	interp *Interp
	vm     moargo.Engine
}

// NewBridge creates a new MoarVM-Tcl bridge and registers commands.
func NewBridge(in *Interp, vm moargo.Engine) *Bridge {
	b := &Bridge{
		interp: in,
		vm:     vm,
	}
	b.register()
	return b
}

func (b *Bridge) register() {
	b.interp.RegisterCommand("moar::init", b.cmdInit)
	b.interp.RegisterCommand("moar::destroy", b.cmdDestroy)
	b.interp.RegisterCommand("moar::state", b.cmdState)
	b.interp.RegisterCommand("moar::run", b.cmdRun)
	b.interp.RegisterCommand("moar::set_prog_name", b.cmdSetProgName)
	b.interp.RegisterCommand("moar::set_args", b.cmdSetArgs)
	b.interp.RegisterCommand("moar::set_lib_paths", b.cmdSetLibPaths)
}

func (b *Bridge) cmdInit(in *Interp, args []string) (string, error) {
	if b.vm == nil {
		return "", fmt.Errorf("moar::init: no VM engine configured")
	}
	if b.vm.State() == moargo.StateReady {
		return "OK", nil
	}
	if err := b.vm.Init(context.Background()); err != nil {
		return "", fmt.Errorf("moar::init failed: %w", err)
	}
	return "OK", nil
}


func (b *Bridge) cmdDestroy(in *Interp, args []string) (string, error) {
	if b.vm == nil {
		return "", fmt.Errorf("moar::destroy: no VM engine configured")
	}
	if err := b.vm.Destroy(); err != nil {
		return "", fmt.Errorf("moar::destroy failed: %w", err)
	}
	return "OK", nil
}

func (b *Bridge) cmdState(in *Interp, args []string) (string, error) {
	if b.vm == nil {
		return "NONE", nil
	}
	return b.vm.State().String(), nil
}

func (b *Bridge) cmdRun(in *Interp, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("wrong # args: should be \"moar::run filePath ?arg ...?\"")
	}
	if b.vm == nil {
		return "", fmt.Errorf("moar::run: no VM engine configured")
	}

	filePath := args[0]
	if len(args) > 1 {
		_ = b.vm.SetArgs(args[1:])
	}

	if err := b.vm.RunFile(context.Background(), filePath); err != nil {
		return "", fmt.Errorf("moar::run failed: %w", err)
	}
	return "OK", nil
}

func (b *Bridge) cmdSetProgName(in *Interp, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("wrong # args: should be \"moar::set_prog_name name\"")
	}
	if err := b.vm.SetProgName(args[0]); err != nil {
		return "", err
	}
	return "OK", nil
}

func (b *Bridge) cmdSetArgs(in *Interp, args []string) (string, error) {
	var argList []string
	if len(args) == 1 {
		argList = strings.Fields(args[0])
	} else {
		argList = args
	}
	if err := b.vm.SetArgs(argList); err != nil {
		return "", err
	}
	return "OK", nil
}

func (b *Bridge) cmdSetLibPaths(in *Interp, args []string) (string, error) {
	var pathList []string
	if len(args) == 1 {
		pathList = strings.Fields(args[0])
	} else {
		pathList = args
	}
	if err := b.vm.SetLibPaths(pathList); err != nil {
		return "", err
	}
	return "OK", nil
}
