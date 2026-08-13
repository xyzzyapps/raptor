# Raptor Structs, Function Pointers & C FFI (NativeCall)

Raptor uses `struct` as its sole compound record type, guaranteeing C-ABI memory layouts for high-performance computing, NativeCall FFI, and hardware OpenGL rendering with Raylib.

---

## 1. C-ABI Struct Declarations

Structs define field types and offsets matching native C compilers:

```perl
struct Point {
    int32 $x;
    int32 $y;
}

my $p = Point.new();
$p.x = 100;
$p.y = 200;
```

---

## 2. Function Pointers & Closures in Structs

Struct fields can store native function pointers or Raptor closures:

```perl
struct Button {
    int32 $x;
    int32 $y;
    ptr $onClick;
}

my $btn = Button.new();
$btn.x = 50;
$btn.y = 100;

# Assign a closure to the struct field
$btn.onClick = sub ($val) {
    say "Button triggered with code: " ~ $val;
};

# Invoke directly through the struct
$btn.onClick(123);
```

---

## 3. Custom Operator Overloading on Structs

Overload mathematical or logical operators for structs with multiple dispatch:

```perl
struct Vec2 {
    num64 $x;
    num64 $y;
}

multi sub infix:<+>(Vec2 $a, Vec2 $b) {
    my $res = Vec2.new();
    $res.x = $a.x + $b.x;
    $res.y = $a.y + $b.y;
    return $res;
}

multi sub infix:<*>(Vec2 $v, $scale) {
    my $res = Vec2.new();
    $res.x = $v.x * $scale;
    $res.y = $v.y * $scale;
    return $res;
}

my $v1 = Vec2.new(); $v1.x = 10.0; $v1.y = 20.0;
my $v2 = Vec2.new(); $v2.x = 5.0;  $v2.y = 15.0;
my $v3 = $v1 + $v2;
my $v4 = $v1 * 2.5;
```

---

## 4. NativeCall C FFI & Raylib 5.5 Graphics

Bind directly to C shared libraries (`.dll`, `.so`, `.dylib`):

```perl
# NativeCall bindings
sub InitWindow(int32 $w, int32 $h, Str $title) returns void is native('libraylib.dll') { * }
sub WindowShouldClose() returns bool is native('libraylib.dll') { * }
sub CloseWindow() returns void is native('libraylib.dll') { * }
sub BeginDrawing() returns void is native('libraylib.dll') { * }
sub EndDrawing() returns void is native('libraylib.dll') { * }
```

Run the complete 60 FPS interactive desktop game:
```powershell
raptor run examples/raylib_game.rp
```
