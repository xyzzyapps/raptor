package raptor

import (
	"runtime"
	"testing"
)

func TestFFIStructGetSystemTime(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows-specific kernel32 FFI test on non-windows platform")
	}
	in := NewInterp()
	code := `
my $k32 = ffi_load("kernel32.dll");
ffi_bind($k32, "GetSystemTime", "void", ["ptr"], "get_sys_time");

# Allocate 16 bytes for SYSTEMTIME struct
my $st = ffi_alloc(16);
get_sys_time($st);

# Read struct fields (wYear at 0, wMonth at 2, wDay at 6, wHour at 8, wMinute at 10, wSecond at 12)
my $year   = ffi_read_uint16($st, 0);
my $month  = ffi_read_uint16($st, 2);
my $day    = ffi_read_uint16($st, 6);
my $hour   = ffi_read_uint16($st, 8);
my $minute = ffi_read_uint16($st, 10);

ffi_free($st);
ffi_close($k32);

[$year, $month, $day, $hour, $minute];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 5 {
		t.Fatalf("expected array of 5 time fields, got %v", val)
	}

	year := val.ArrayVal[0].IntVal
	month := val.ArrayVal[1].IntVal
	day := val.ArrayVal[2].IntVal

	if year < 2024 || year > 2030 {
		t.Errorf("unexpected year: %d", year)
	}
	if month < 1 || month > 12 {
		t.Errorf("unexpected month: %d", month)
	}
	if day < 1 || day > 31 {
		t.Errorf("unexpected day: %d", day)
	}
}

func TestFFIStructWriteAndRead(t *testing.T) {
	in := NewInterp()
	code := `
# Allocate custom struct: int32 x, int32 y, float64 score, int64 id
my $buf = ffi_alloc(24);
ffi_write_int32($buf, 0, 100);
ffi_write_int32($buf, 4, 200);
ffi_write_float64($buf, 8, 98.5);
ffi_write_int64($buf, 16, 9999999999);

my $x = ffi_read_int32($buf, 0);
my $y = ffi_read_int32($buf, 4);
my $score = ffi_read_float64($buf, 8);
my $id = ffi_read_int64($buf, 16);

ffi_free($buf);
[$x, $y, $score, $id];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 4 {
		t.Fatalf("expected array of 4, got %v", val)
	}
	if val.ArrayVal[0].IntVal != 100 {
		t.Errorf("x mismatch: %d", val.ArrayVal[0].IntVal)
	}
	if val.ArrayVal[1].IntVal != 200 {
		t.Errorf("y mismatch: %d", val.ArrayVal[1].IntVal)
	}
	if val.ArrayVal[2].FloatVal != 98.5 {
		t.Errorf("score mismatch: %f", val.ArrayVal[2].FloatVal)
	}
	if val.ArrayVal[3].IntVal != 9999999999 {
		t.Errorf("id mismatch: %d", val.ArrayVal[3].IntVal)
	}
}
