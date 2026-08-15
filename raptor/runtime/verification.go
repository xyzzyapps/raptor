package raptor

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

func registerVerificationBuiltins(in *Interp) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	evalCond := func(in *Interp, arg *Value) (bool, error) {
		if arg == nil {
			return false, nil
		}
		if arg.Type == ValClosure {
			res, err := in.InvokeCallable(arg, nil)
			if err != nil {
				return false, err
			}
			return res.IsTrue(), nil
		}
		return arg.IsTrue(), nil
	}

	// PRE / pre($condition, [$message]) - Precondition contract
	preHandler := func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("PreconditionError: precondition failed")
		}
		ok, err := evalCond(in, args[0])
		if err != nil {
			return nil, fmt.Errorf("PreconditionError evaluation failed: %w", err)
		}
		if !ok {
			msg := "precondition failed"
			if len(args) > 1 {
				msg = args[1].String()
			}
			return nil, fmt.Errorf("PreconditionError: %s", msg)
		}
		return BoolValue(true), nil
	}
	in.Builtins["PRE"] = preHandler
	in.Builtins["pre"] = preHandler

	// POST / post($condition, [$message]) - Postcondition contract
	postHandler := func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("PostconditionError: postcondition failed")
		}
		ok, err := evalCond(in, args[0])
		if err != nil {
			return nil, fmt.Errorf("PostconditionError evaluation failed: %w", err)
		}
		if !ok {
			msg := "postcondition failed"
			if len(args) > 1 {
				msg = args[1].String()
			}
			return nil, fmt.Errorf("PostconditionError: %s", msg)
		}
		return BoolValue(true), nil
	}
	in.Builtins["POST"] = postHandler
	in.Builtins["post"] = postHandler

	// INVARIANT / invariant($condition, [$message]) - Invariant contract
	invariantHandler := func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("InvariantError: invariant violation")
		}
		ok, err := evalCond(in, args[0])
		if err != nil {
			return nil, fmt.Errorf("InvariantError evaluation failed: %w", err)
		}
		if !ok {
			msg := "invariant violation"
			if len(args) > 1 {
				msg = args[1].String()
			}
			return nil, fmt.Errorf("InvariantError: %s", msg)
		}
		return BoolValue(true), nil
	}
	in.Builtins["INVARIANT"] = invariantHandler
	in.Builtins["invariant"] = invariantHandler

	// CHECK / check / ASSERT / assert ($condition, [$message]) - Explicit assertion
	checkHandler := func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("AssertionError: assertion failed")
		}
		ok, err := evalCond(in, args[0])
		if err != nil {
			return nil, fmt.Errorf("AssertionError evaluation failed: %w", err)
		}
		if !ok {
			msg := "assertion failed"
			if len(args) > 1 {
				msg = args[1].String()
			}
			return nil, fmt.Errorf("AssertionError: %s", msg)
		}
		return BoolValue(true), nil
	}
	in.Builtins["CHECK"] = checkHandler
	in.Builtins["check"] = checkHandler
	in.Builtins["ASSERT"] = checkHandler
	in.Builtins["assert"] = checkHandler

	// TEST / test ($name, $closure) - Zero-overhead inline tests
	inlineTestHandler := func(in *Interp, args []*Value) (*Value, error) {
		if os.Getenv("RAPTOR_TEST_MODE") != "1" {
			return NilValue(), nil
		}
		if len(args) < 2 || args[1].Type != ValClosure {
			return nil, fmt.Errorf("TEST requires a description string and test closure")
		}
		name := args[0].String()
		closure := args[1]
		if subtestFn, ok := in.Builtins["subtest"]; ok {
			return subtestFn(in, []*Value{StringValue(name), closure})
		}
		return NilValue(), nil
	}
	in.Builtins["TEST"] = inlineTestHandler
	in.Builtins["test"] = inlineTestHandler

	// SUBTEST / subtest ($name, $closure) - Nested test suite
	subtestAlias := func(in *Interp, args []*Value) (*Value, error) {
		if subtestFn, ok := in.Builtins["subtest"]; ok {
			return subtestFn(in, args)
		}
		return NilValue(), nil
	}
	in.Builtins["SUBTEST"] = subtestAlias

	// PROPERTY / property ($name, $closure, [%opts]) - Property-Based QuickCheck Testing / Fuzzing
	propertyHandler := func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[1].Type != ValClosure {
			return nil, fmt.Errorf("PROPERTY requires name string and verification closure")
		}
		name := args[0].String()
		closure := args[1].ClosureVal
		iterations := 100
		if len(args) > 2 && args[2].Type == ValHash {
			if countOpt, ok := args[2].HashVal["iterations"]; ok {
				iterations = int(in.toInt(countOpt))
			}
		}

		paramCount := len(closure.Params)
		for i := 0; i < iterations; i++ {
			// Generate randomized test vector
			var sampleArgs []*Value
			for pIdx := 0; pIdx < paramCount; pIdx++ {
				switch pIdx % 4 {
				case 0: // Integers (including negatives, zero, boundaries)
					rInt := rng.Int63n(2000) - 1000
					if i%10 == 0 {
						rInt = 0
					}
					sampleArgs = append(sampleArgs, IntValue(rInt))
				case 1: // Small positive integers
					sampleArgs = append(sampleArgs, IntValue(rng.Int63n(100)+1))
				case 2: // Strings
					strs := []string{"", "a", "raptor", "test_123", "hello world", "αβγ"}
					sampleArgs = append(sampleArgs, StringValue(strs[rng.Intn(len(strs))]))
				case 3: // Floats
					sampleArgs = append(sampleArgs, FloatValue(rng.Float64()*100.0-50.0))
				}
			}

			// Invoke property check
			res, err := in.InvokeCallable(args[1], sampleArgs)
			if err != nil || (res != nil && !res.IsTrue()) {
				var argStrs []string
				for _, a := range sampleArgs {
					argStrs = append(argStrs, a.String())
				}
				failMsg := fmt.Sprintf("Property %q falsified on iteration %d with arguments: (%s)",
					name, i+1, strings.Join(argStrs, ", "))
				if err != nil {
					failMsg += fmt.Sprintf(" (error: %v)", err)
				}
				if okFn, ok := in.Builtins["ok"]; ok {
					return okFn(in, []*Value{BoolValue(false), StringValue(failMsg)})
				}
				return BoolValue(false), fmt.Errorf("%s", failMsg)
			}
		}

		successMsg := fmt.Sprintf("Property %q holds for %d randomized trials", name, iterations)
		if okFn, ok := in.Builtins["ok"]; ok {
			return okFn(in, []*Value{BoolValue(true), StringValue(successMsg)})
		}
		return BoolValue(true), nil
	}
	in.Builtins["PROPERTY"] = propertyHandler
	in.Builtins["property"] = propertyHandler
}
