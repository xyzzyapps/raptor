package raptor

import (
	"encoding/json"
	"fmt"
)

func (in *Interp) registerJSONBuiltins() {
	// to_json(val, [pretty]) / to-json / json_encode
	toJSONHandler := func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("to_json requires a value argument")
		}
		rawObj := valueToInterface(args[0])
		pretty := false
		if len(args) >= 2 {
			pretty = args[1].IsTrue()
		}

		var bytes []byte
		var err error
		if pretty {
			bytes, err = json.MarshalIndent(rawObj, "", "  ")
		} else {
			bytes, err = json.Marshal(rawObj)
		}
		if err != nil {
			return nil, fmt.Errorf("json serialization error: %w", err)
		}
		return StringValue(string(bytes)), nil
	}

	in.Builtins["to_json"] = toJSONHandler
	in.Builtins["to-json"] = toJSONHandler
	in.Builtins["json_encode"] = toJSONHandler

	// from_json(str) / from-json / json_decode
	fromJSONHandler := func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("from_json requires a JSON string argument")
		}
		jsonStr := args[0].String()
		var raw any
		if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
			return nil, fmt.Errorf("json parse error: %w", err)
		}
		return interfaceToValue(raw), nil
	}

	in.Builtins["from_json"] = fromJSONHandler
	in.Builtins["from-json"] = fromJSONHandler
	in.Builtins["json_decode"] = fromJSONHandler
}

func valueToInterface(v *Value) any {
	if v == nil {
		return nil
	}
	switch v.Type {
	case ValNil:
		return nil
	case ValBool:
		return v.BoolVal
	case ValInt:
		return v.IntVal
	case ValFloat:
		return v.FloatVal
	case ValString:
		return v.StrVal
	case ValArray:
		if v.ArrayVal == nil {
			if ints := v.intLane(); ints != nil {
				list := make([]any, len(ints))
				for i, n := range ints {
					list[i] = n
				}
				return list
			}
			if floats := v.floatLane(); floats != nil {
				list := make([]any, len(floats))
				for i, f := range floats {
					list[i] = f
				}
				return list
			}
			if strs := v.strLane(); strs != nil {
				n := strs.Len()
				list := make([]any, n)
				for i := 0; i < n; i++ {
					list[i] = strs.At(i)
				}
				return list
			}
		}
		list := make([]any, 0, len(v.ArrayVal))
		for _, elem := range v.ArrayVal {
			list = append(list, valueToInterface(elem))
		}
		return list
	case ValHash:
		dict := make(map[string]any)
		for k, val := range v.HashVal {
			dict[k] = valueToInterface(val)
		}
		return dict
	case ValLazySeq:
		if ls := v.LazySeq(); ls != nil {
			var list []any
			for _, elem := range ls.Items {
				list = append(list, valueToInterface(elem))
			}
			return list
		}
		return []any{}
	default:
		return v.String()
	}
}

func interfaceToValue(v any) *Value {
	if v == nil {
		return NilValue()
	}
	switch val := v.(type) {
	case bool:
		return BoolValue(val)
	case float64:
		if val == float64(int64(val)) {
			return IntValue(int64(val))
		}
		return FloatValue(val)
	case string:
		return StringValue(val)
	case []any:
		var list []*Value
		for _, elem := range val {
			list = append(list, interfaceToValue(elem))
		}
		return ArrayValue(list)
	case map[string]any:
		m := make(map[string]*Value)
		for k, vItem := range val {
			m[k] = interfaceToValue(vItem)
		}
		return HashValue(m)
	default:
		return StringValue(fmt.Sprintf("%v", val))
	}
}
