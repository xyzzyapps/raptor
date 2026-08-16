//go:build !js || !wasm

package raptor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (in *Interp) registerPerl5Bridge() {
	in.Builtins["eval_perl5"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return NilValue(), nil
		}
		script := args[0].String()
		return in.runPerl5Eval(script)
	}

	in.Builtins["call_perl5"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("call_perl5 requires sub name")
		}
		subName := args[0].String()
		var callArgs []any
		for _, a := range args[1:] {
			callArgs = append(callArgs, a.ToInterface())
		}
		return in.callPerl5Sub(subName, callArgs)
	}

	in.Builtins["require_perl5"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("require_perl5 requires module name")
		}
		mod := args[0].String()
		return in.runPerl5Eval(fmt.Sprintf("require %s; 1;", mod))
	}
}

func (in *Interp) evalUse(u *UseStmt, env *Env) (*Value, error) {
	if u.From == "Perl5" {
		_, err := in.runPerl5Eval(fmt.Sprintf("use %s; 1;", u.Module))
		if err != nil {
			return nil, fmt.Errorf("failed loading Perl 5 module %s: %w", u.Module, err)
		}
		return BoolValue(true), nil
	}

	// 1. Convert Package::Submodule to filesystem path
	u.Module = strings.Trim(u.Module, `"'`)
	relPath := strings.ReplaceAll(u.Module, "::", "/")
	baseName := filepath.Base(relPath)

	candidates := []string{
		u.Module,
		filepath.Join("lib", relPath+".rp"),
		filepath.Join("lib", relPath+".raptor"),
		filepath.Join("lib", relPath, baseName+".rp"),
		relPath + ".rp",
		relPath + ".raptor",
		relPath + ".moarvm",
		filepath.Join("lib", relPath+".moarvm"),
	}

	// 2. Search raptor_modules/ (including examples/raptor_modules/)
	if rmPath := FindModuleInRaptorModules(baseName); rmPath != "" {
		candidates = append([]string{rmPath}, candidates...)
	}
	if _, err := os.Stat("examples/raptor_modules"); err == nil {
		_ = filepath.Walk("examples/raptor_modules", func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				b := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
				if (b == baseName || b == u.Module) && (strings.HasSuffix(p, ".rp") || strings.HasSuffix(p, ".raptor")) {
					candidates = append([]string{p}, candidates...)
					return filepath.SkipAll
				}
			}
			return nil
		})
	}

	// 3. Evaluate matching candidate
	for _, path := range candidates {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			if strings.HasSuffix(path, ".moarvm") {
				if loader, ok := in.Builtins["moar_load_module"]; ok {
					return loader(in, []*Value{StringValue(path)})
				}
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed reading module %s from %s: %w", u.Module, path, err)
			}
			prevPkg := in.CurrentPackage
			in.CurrentPackage = u.Module
			_ = in.GetPackage(u.Module)
			res, evalErr := in.Eval(string(content))
			in.CurrentPackage = prevPkg
			return res, evalErr
		}
	}

	return BoolValue(true), nil
}

func (in *Interp) runPerl5Eval(code string) (*Value, error) {
	perlScript := fmt.Sprintf(`
use strict;
use warnings;
use JSON::PP;

my $res = eval {
    %s
};
if ($@) {
    print encode_json({ error => "$@" });
} else {
    print encode_json({ result => $res });
}
`, code)

	perlBin := "perl"
	cmd := exec.Command(perlBin, "-e", perlScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("perl execution failed: %w", err)
	}

	var resp struct {
		Result any    `json:"result"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		s := strings.TrimSpace(string(out))
		return StringValue(s), nil
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("perl error: %s", resp.Error)
	}

	return toRakuValue(resp.Result), nil
}

func (in *Interp) callPerl5Sub(subName string, args []any) (*Value, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	perlScript := fmt.Sprintf(`
use strict;
use warnings;
use JSON::PP;

my $args_raw = '%s';
my $args = decode_json($args_raw);

my $sub_name = '%s';
my $res;

if ($sub_name =~ /::/) {
    my ($mod) = $sub_name =~ /^(.*)::/;
    eval "require $mod;" if $mod;
}

my $code = \&{"$sub_name"};
if ($code) {
    $res = eval { $code->(@$args) };
    if ($@) {
        print encode_json({ error => "$@" });
        exit 0;
    }
} else {
    print encode_json({ error => "Undefined Perl subroutine $sub_name" });
    exit 0;
}

print encode_json({ result => $res });
`, escapePerlSingleQuote(string(argsJSON)), subName)

	cmd := exec.Command("perl", "-e", perlScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("perl call failed: %w", err)
	}

	var resp struct {
		Result any    `json:"result"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return StringValue(strings.TrimSpace(string(out))), nil
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("perl error: %s", resp.Error)
	}

	return toRakuValue(resp.Result), nil
}

func toRakuValue(val any) *Value {
	if val == nil {
		return NilValue()
	}
	switch v := val.(type) {
	case bool:
		return BoolValue(v)
	case float64:
		if v == float64(int64(v)) {
			return IntValue(int64(v))
		}
		return FloatValue(v)
	case string:
		return StringValue(v)
	case []any:
		var list []*Value
		for _, item := range v {
			list = append(list, toRakuValue(item))
		}
		return ArrayValue(list)
	case map[string]any:
		m := make(map[string]*Value)
		for k, item := range v {
			m[k] = toRakuValue(item)
		}
		return HashValue(m)
	default:
		return StringValue(fmt.Sprintf("%v", v))
	}
}

func escapePerlSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}
