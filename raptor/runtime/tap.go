package raptor

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

// TAPState maintains the state of the active TAP producer.
type TAPState struct {
	mu          sync.Mutex
	TestCount   int
	Planned     int
	FailedCount int
	DonePlanned bool
	IndentLevel int
	Out         io.Writer
}

func newTAPState(out io.Writer) *TAPState {
	if out == nil {
		out = os.Stdout
	}
	return &TAPState{
		Out: out,
	}
}

func (s *TAPState) indent() string {
	if s.IndentLevel <= 0 {
		return ""
	}
	return strings.Repeat("    ", s.IndentLevel)
}

func registerTAPBuiltins(in *Interp) {
	st := newTAPState(in.Stdout)

	// plan($count)
	in.Builtins["plan"] = func(in *Interp, args []*Value) (*Value, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.Out = in.Stdout
		if len(args) == 0 {
			return NilValue(), nil
		}
		st.Planned = int(in.toInt(args[0]))
		st.DonePlanned = true
		fmt.Fprintf(st.Out, "%s1..%d\n", st.indent(), st.Planned)
		return IntValue(int64(st.Planned)), nil
	}

	// ok($cond, [$name])
	in.Builtins["ok"] = func(in *Interp, args []*Value) (*Value, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.Out = in.Stdout
		st.TestCount++
		cond := false
		if len(args) > 0 {
			cond = args[0].IsTrue()
		}
		name := ""
		if len(args) > 1 && args[1].Type != ValNil {
			name = " - " + args[1].String()
		}

		if cond {
			fmt.Fprintf(st.Out, "%sok %d%s\n", st.indent(), st.TestCount, name)
			return BoolValue(true), nil
		}
		st.FailedCount++
		fmt.Fprintf(st.Out, "%snot ok %d%s\n", st.indent(), st.TestCount, name)
		fmt.Fprintf(st.Out, "%s#   Failed test%s\n", st.indent(), name)
		return BoolValue(false), nil
	}

	// is($got, $expected, [$name])
	in.Builtins["is"] = func(in *Interp, args []*Value) (*Value, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.Out = in.Stdout
		st.TestCount++

		var got, expected *Value = NilValue(), NilValue()
		if len(args) > 0 {
			got = args[0]
		}
		if len(args) > 1 {
			expected = args[1]
		}
		name := ""
		if len(args) > 2 && args[2].Type != ValNil {
			name = " - " + args[2].String()
		}

		passed := in.compareValues(got, expected) == 0
		if passed {
			fmt.Fprintf(st.Out, "%sok %d%s\n", st.indent(), st.TestCount, name)
			return BoolValue(true), nil
		}

		st.FailedCount++
		fmt.Fprintf(st.Out, "%snot ok %d%s\n", st.indent(), st.TestCount, name)
		fmt.Fprintf(st.Out, "%s#   Failed test%s\n", st.indent(), name)
		fmt.Fprintf(st.Out, "%s#          got: %s\n", st.indent(), got.String())
		fmt.Fprintf(st.Out, "%s#     expected: %s\n", st.indent(), expected.String())
		return BoolValue(false), nil
	}

	// isnt($got, $expected, [$name])
	in.Builtins["isnt"] = func(in *Interp, args []*Value) (*Value, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.Out = in.Stdout
		st.TestCount++

		var got, expected *Value = NilValue(), NilValue()
		if len(args) > 0 {
			got = args[0]
		}
		if len(args) > 1 {
			expected = args[1]
		}
		name := ""
		if len(args) > 2 && args[2].Type != ValNil {
			name = " - " + args[2].String()
		}

		passed := in.compareValues(got, expected) != 0
		if passed {
			fmt.Fprintf(st.Out, "%sok %d%s\n", st.indent(), st.TestCount, name)
			return BoolValue(true), nil
		}

		st.FailedCount++
		fmt.Fprintf(st.Out, "%snot ok %d%s\n", st.indent(), st.TestCount, name)
		fmt.Fprintf(st.Out, "%s#   Failed test%s (got matching value %s)\n", st.indent(), name, got.String())
		return BoolValue(false), nil
	}

	// is_deeply($got, $expected, [$name])
	in.Builtins["is_deeply"] = func(in *Interp, args []*Value) (*Value, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.Out = in.Stdout
		st.TestCount++

		var got, expected *Value = NilValue(), NilValue()
		if len(args) > 0 {
			got = args[0]
		}
		if len(args) > 1 {
			expected = args[1]
		}
		name := ""
		if len(args) > 2 && args[2].Type != ValNil {
			name = " - " + args[2].String()
		}

		passed := valuesDeepEqual(got, expected)
		if passed {
			fmt.Fprintf(st.Out, "%sok %d%s\n", st.indent(), st.TestCount, name)
			return BoolValue(true), nil
		}

		st.FailedCount++
		fmt.Fprintf(st.Out, "%snot ok %d%s\n", st.indent(), st.TestCount, name)
		fmt.Fprintf(st.Out, "%s#   Failed test (structures differ)%s\n", st.indent(), name)
		fmt.Fprintf(st.Out, "%s#          got: %s\n", st.indent(), got.String())
		fmt.Fprintf(st.Out, "%s#     expected: %s\n", st.indent(), expected.String())
		return BoolValue(false), nil
	}

	// like($got, $regex, [$name])
	in.Builtins["like"] = func(in *Interp, args []*Value) (*Value, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.Out = in.Stdout
		st.TestCount++

		gotStr := ""
		pattern := ""
		if len(args) > 0 {
			gotStr = args[0].String()
		}
		if len(args) > 1 {
			pattern = args[1].String()
		}
		name := ""
		if len(args) > 2 && args[2].Type != ValNil {
			name = " - " + args[2].String()
		}

		matched, err := regexp.MatchString(pattern, gotStr)
		passed := err == nil && matched

		if passed {
			fmt.Fprintf(st.Out, "%sok %d%s\n", st.indent(), st.TestCount, name)
			return BoolValue(true), nil
		}

		st.FailedCount++
		fmt.Fprintf(st.Out, "%snot ok %d%s\n", st.indent(), st.TestCount, name)
		fmt.Fprintf(st.Out, "%s#   Failed test '%s' doesn't match '%s'%s\n", st.indent(), gotStr, pattern, name)
		return BoolValue(false), nil
	}

	// unlike($got, $regex, [$name])
	in.Builtins["unlike"] = func(in *Interp, args []*Value) (*Value, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.Out = in.Stdout
		st.TestCount++

		gotStr := ""
		pattern := ""
		if len(args) > 0 {
			gotStr = args[0].String()
		}
		if len(args) > 1 {
			pattern = args[1].String()
		}
		name := ""
		if len(args) > 2 && args[2].Type != ValNil {
			name = " - " + args[2].String()
		}

		matched, err := regexp.MatchString(pattern, gotStr)
		passed := err == nil && !matched

		if passed {
			fmt.Fprintf(st.Out, "%sok %d%s\n", st.indent(), st.TestCount, name)
			return BoolValue(true), nil
		}

		st.FailedCount++
		fmt.Fprintf(st.Out, "%snot ok %d%s\n", st.indent(), st.TestCount, name)
		fmt.Fprintf(st.Out, "%s#   Failed test '%s' unexpectedly matches '%s'%s\n", st.indent(), gotStr, pattern, name)
		return BoolValue(false), nil
	}

	// cmp_ok($got, $op, $expected, [$name])
	in.Builtins["cmp_ok"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("cmp_ok requires got, op, expected")
		}
		got := args[0]
		op := args[1].String()
		expected := args[2]
		name := ""
		if len(args) > 3 {
			name = args[3].String()
		}

		res, err := in.evalBinaryOp(got, op, expected)
		if err != nil {
			return nil, err
		}
		return in.Builtins["ok"](in, []*Value{res, StringValue(name)})
	}

	// pass([$name])
	in.Builtins["pass"] = func(in *Interp, args []*Value) (*Value, error) {
		name := ""
		if len(args) > 0 {
			name = args[0].String()
		}
		return in.Builtins["ok"](in, []*Value{BoolValue(true), StringValue(name)})
	}

	// fail([$name])
	in.Builtins["fail"] = func(in *Interp, args []*Value) (*Value, error) {
		name := ""
		if len(args) > 0 {
			name = args[0].String()
		}
		return in.Builtins["ok"](in, []*Value{BoolValue(false), StringValue(name)})
	}

	// diag($msg)
	in.Builtins["diag"] = func(in *Interp, args []*Value) (*Value, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.Out = in.Stdout
		msg := ""
		if len(args) > 0 {
			msg = args[0].String()
		}
		for _, l := range strings.Split(msg, "\n") {
			fmt.Fprintf(st.Out, "%s# %s\n", st.indent(), l)
		}
		return NilValue(), nil
	}

	// subtest($name, $closure)
	in.Builtins["subtest"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[1].Type != ValClosure {
			return nil, fmt.Errorf("subtest requires a name and a closure")
		}
		name := args[0].String()
		closure := args[1]

		st.mu.Lock()
		st.Out = in.Stdout
		fmt.Fprintf(st.Out, "%s# Subtest: %s\n", st.indent(), name)
		st.IndentLevel++
		prevCount := st.TestCount
		prevFailed := st.FailedCount
		st.TestCount = 0
		st.FailedCount = 0
		st.mu.Unlock()

		// Run subtest body
		_, err := in.InvokeCallable(closure, nil)

		st.mu.Lock()
		subPass := st.FailedCount == 0
		st.IndentLevel--
		st.TestCount = prevCount + 1
		st.FailedCount = prevFailed
		if !subPass || err != nil {
			st.FailedCount++
			fmt.Fprintf(st.Out, "%snot ok %d - %s\n", st.indent(), st.TestCount, name)
		} else {
			fmt.Fprintf(st.Out, "%sok %d - %s\n", st.indent(), st.TestCount, name)
		}
		st.mu.Unlock()

		return BoolValue(subPass && err == nil), nil
	}

	// done_testing([$count])
	in.Builtins["done_testing"] = func(in *Interp, args []*Value) (*Value, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.Out = in.Stdout
		if !st.DonePlanned {
			fmt.Fprintf(st.Out, "%s1..%d\n", st.indent(), st.TestCount)
			st.DonePlanned = true
			st.Planned = st.TestCount
		}
		passed := st.FailedCount == 0 && (st.Planned == 0 || st.Planned == st.TestCount)
		return BoolValue(passed), nil
	}

	// tap_summary() -> %hash of stats
	in.Builtins["tap_summary"] = func(in *Interp, args []*Value) (*Value, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		m := map[string]*Value{
			"total":   IntValue(int64(st.TestCount)),
			"failed":  IntValue(int64(st.FailedCount)),
			"passed":  IntValue(int64(st.TestCount - st.FailedCount)),
			"planned": IntValue(int64(st.Planned)),
		}
		return HashValue(m), nil
	}
}

// valuesDeepEqual checks recursive structural equality of two Value instances
func valuesDeepEqual(a, b *Value) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Type != b.Type {
		return false
	}

	switch a.Type {
	case ValInt:
		return a.IntVal == b.IntVal
	case ValFloat:
		return a.FloatVal == b.FloatVal
	case ValString:
		return a.StrVal == b.StrVal
	case ValBool:
		return a.BoolVal == b.BoolVal
	case ValArray:
		if len(a.ArrayVal) != len(b.ArrayVal) {
			return false
		}
		for i := range a.ArrayVal {
			if !valuesDeepEqual(a.ArrayVal[i], b.ArrayVal[i]) {
				return false
			}
		}
		return true
	case ValHash:
		if len(a.HashVal) != len(b.HashVal) {
			return false
		}
		for k, vA := range a.HashVal {
			vB, ok := b.HashVal[k]
			if !ok || !valuesDeepEqual(vA, vB) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}
