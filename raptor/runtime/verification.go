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

	// pre($condition, [$message]) - Precondition contract
	in.Builtins["pre"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 || !args[0].IsTrue() {
			msg := "precondition failed"
			if len(args) > 1 {
				msg = args[1].String()
			}
			return nil, fmt.Errorf("PreconditionError: %s", msg)
		}
		return BoolValue(true), nil
	}

	// post($condition, [$message]) - Postcondition contract
	in.Builtins["post"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 || !args[0].IsTrue() {
			msg := "postcondition failed"
			if len(args) > 1 {
				msg = args[1].String()
			}
			return nil, fmt.Errorf("PostconditionError: %s", msg)
		}
		return BoolValue(true), nil
	}

	// invariant($condition, [$message]) - Invariant contract
	in.Builtins["invariant"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 || !args[0].IsTrue() {
			msg := "invariant violation"
			if len(args) > 1 {
				msg = args[1].String()
			}
			return nil, fmt.Errorf("InvariantError: %s", msg)
		}
		return BoolValue(true), nil
	}

	// TEST($name, $closure) or test($name, $closure) - Zero-overhead inline tests
	// Only runs when RAPTOR_TEST_MODE is enabled or --test flag is set
	inlineTestHandler := func(in *Interp, args []*Value) (*Value, error) {
		if os.Getenv("RAPTOR_TEST_MODE") != "1" {
			return NilValue(), nil
		}
		if len(args) < 2 || args[1].Type != ValClosure {
			return nil, fmt.Errorf("TEST requires a description string and test closure")
		}
		name := args[0].String()
		closure := args[1]
		return in.Builtins["subtest"](in, []*Value{StringValue(name), closure})
	}

	in.Builtins["TEST"] = inlineTestHandler
	in.Builtins["test"] = inlineTestHandler

	// property($name, $closure, [%opts]) - Property-Based QuickCheck Testing / Fuzzing
	in.Builtins["property"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[1].Type != ValClosure {
			return nil, fmt.Errorf("property requires name string and verification closure")
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
				return in.Builtins["ok"](in, []*Value{BoolValue(false), StringValue(failMsg)})
			}
		}

		successMsg := fmt.Sprintf("Property %q holds for %d randomized trials", name, iterations)
		return in.Builtins["ok"](in, []*Value{BoolValue(true), StringValue(successMsg)})
	}
}
