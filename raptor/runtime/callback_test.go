package raptor

import (
	"runtime"
	"testing"
)

func TestFFICallbackCreationAndDispatch(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows-specific syscall callback test on non-windows platform")
	}
	in := NewInterp()
	code := `
my $invoked = 0;
my $received_arg = 0;

my $cb = ffi_callback(sub ($a, $b, $c, $d) {
    $invoked = 1;
    $received_arg = $a;
    return $a * 2;
});

# Verify callback is a non-zero C function pointer
my $is_valid_ptr = $cb > 0;

[$is_valid_ptr, $cb];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("expected array of 2 elements, got %v", val)
	}
	if !val.ArrayVal[0].IsTrue() {
		t.Errorf("callback pointer was not valid: %v", val.ArrayVal[1])
	}
}

func TestFFIPointerReadWriteAndOpaque(t *testing.T) {
	in := NewInterp()
	code := `
my $mem = ffi_alloc(64);

# Write int64 and float64 at offsets
ffi_write_int64($mem, 0, 123456789);
ffi_write_float64($mem, 8, 3.14159);

my $read_int = ffi_read_int64($mem, 0);
my $read_flt = ffi_read_float64($mem, 8);

# Write and read raw pointer
ffi_write_ptr($mem, 16, $mem);
my $read_ptr = ffi_read_ptr($mem, 16);

ffi_free($mem);

[$read_int, $read_flt > 3.14, $read_ptr == $mem];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("expected array of 3 elements, got %v", val)
	}
	if val.ArrayVal[0].IntVal != 123456789 {
		t.Errorf("int mismatch: %d", val.ArrayVal[0].IntVal)
	}
	if !val.ArrayVal[1].IsTrue() {
		t.Errorf("float comparison failed")
	}
	if !val.ArrayVal[2].IsTrue() {
		t.Errorf("pointer roundtrip failed")
	}
}
