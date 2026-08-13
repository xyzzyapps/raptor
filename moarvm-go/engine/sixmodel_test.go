package moargo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSixModelClassDefinition(t *testing.T) {
	cls := NewSixModelClass("Person", REPR_P6opaque)
	cls.AddAttribute("$!name", RegStr)
	cls.AddAttribute("$!age", RegInt64)

	if cls.Name != "Person" || cls.Repr != REPR_P6opaque {
		t.Fatalf("unexpected class metadata: %+v", cls)
	}
	if len(cls.Attributes) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(cls.Attributes))
	}
}

func TestSixModelBytecodeExecution(t *testing.T) {
	dllPath := filepath.Join("..", "build", "moarvm", "bin", "moar.dll")
	absPath, err := filepath.Abs(dllPath)
	if err != nil {
		t.Fatalf("failed to resolve dll path: %v", err)
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Skip("moar.dll not found, skipping MoarVM test")
	}

	vm, err := New(Config{
		DLLPath:  absPath,
		ProgName: "sixmodel_test",
	})
	if err != nil {
		t.Fatalf("failed creating MoarVM: %v", err)
	}
	ctx := context.Background()
	if err := vm.Init(ctx); err != nil {
		t.Fatalf("failed init MoarVM: %v", err)
	}
	defer vm.Destroy()

	cu := NewCompUnitEmitter("sixmodel_hll")
	f := cu.NewFrame("main", 8)
	f.SetLocalType(0, RegInt64)
	f.SetLocalType(1, RegInt64)
	f.SetLocalType(2, RegInt64)

	// OpConstI64
	f.EmitOp(OpConstI64)
	f.EmitReg(0)
	f.EmitInt64(42)

	// OpConstI64
	f.EmitOp(OpConstI64)
	f.EmitReg(1)
	f.EmitInt64(58)

	// OpAddI
	f.EmitOp(OpAddI)
	f.EmitReg(2)
	f.EmitReg(0)
	f.EmitReg(1)

	// OpReturn
	f.EmitOp(OpReturn)

	bc, err := cu.Emit()
	if err != nil {
		t.Fatalf("failed emitting bytecode: %v", err)
	}

	if err := vm.RunBytecode(ctx, bc); err != nil {
		t.Fatalf("failed running 6model bytecode on MoarVM: %v", err)
	}
}

func TestSerializationContextSerializeDeserialize(t *testing.T) {
	sc := NewSerializationContext("SC_TEST_01", "Test Serialization Context")
	sc.AddDependency("CORE_SC")

	st := NewSTable("KnowHOW", "ClassWHAT", "ClassWHO", REPR_P6opaque)
	st.Methods["greet"] = 1
	st.Methods["farewell"] = 2
	sc.AddSTable(st)

	sc.AddObject([]byte("serialized_object_data_12345"))

	sc.Repossessions = append(sc.Repossessions, Repossession{
		ObjIndex:  0,
		OrigSC:    "CORE_SC",
		OrigIndex: 42,
	})

	data, err := sc.Serialize()
	if err != nil {
		t.Fatalf("SC serialization failed: %v", err)
	}

	deserialized, err := DeserializeSerializationContext(data)
	if err != nil {
		t.Fatalf("SC deserialization failed: %v", err)
	}

	if deserialized.Handle != "SC_TEST_01" {
		t.Fatalf("expected handle 'SC_TEST_01', got %q", deserialized.Handle)
	}
	if len(deserialized.Dependencies) != 1 || deserialized.Dependencies[0] != "CORE_SC" {
		t.Fatalf("unexpected dependencies: %+v", deserialized.Dependencies)
	}
	if len(deserialized.RootSTables) != 1 || deserialized.RootSTables[0].HOWName != "KnowHOW" {
		t.Fatalf("unexpected STables: %+v", deserialized.RootSTables)
	}
	if len(deserialized.RootObjects) != 1 || string(deserialized.RootObjects[0]) != "serialized_object_data_12345" {
		t.Fatalf("unexpected root objects: %+v", deserialized.RootObjects)
	}
	if len(deserialized.Repossessions) != 1 || deserialized.Repossessions[0].OrigIndex != 42 {
		t.Fatalf("unexpected repossessions: %+v", deserialized.Repossessions)
	}
}

