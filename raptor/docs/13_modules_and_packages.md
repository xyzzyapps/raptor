# Raptor Documentation Manual: Modules, Namespaces & Package Management

## 1. Introduction to Modules

Raptor is built on a strictly non-OO procedural paradigm. Code reuse and logical separation are organized around **Modules**, **Subroutines**, and **C-ABI Struct Records**.

A module groups related functions, dynamic refinement types (`subset`), structs, and constants into a cohesive unit.

## 2. Defining a Module

Modules are typically stored in the `./lib/` directory or in `./raptor_modules/` with a `.rp` or `.raptor` extension.

### Example: `lib/Math/Matrix.rp`

```perl
# Module declaration
unit module Math::Matrix;

# Exported struct
struct Matrix2x2 {
    num64 $m00; num64 $m01;
    num64 $m10; num64 $m11;
}

# Exported subroutine
sub matrix_identity() {
    my $m = Matrix2x2.new();
    $m.m00 = 1.0; $m.m01 = 0.0;
    $m.m10 = 0.0; $m.m11 = 1.0;
    return $m;
}

# Exported multi sub with predicate dispatch
multi sub matrix_determinant(Matrix2x2 $m) {
    return ($m.m00 * $m.m11) - ($m.m01 * $m.m10);
}
```

## 3. Importing Modules (`use` & `require`)

### 3.1 The `use` Statement

The `use` statement imports a module at compile/startup time:

```perl
use Math::Matrix;

my $mat = matrix_identity();
say "Determinant: " ~ matrix_determinant($mat);
```

### 3.2 Dynamic Ingestion with `require`

The `require` statement evaluates and loads a script file at runtime:

```perl
require "lib/Config/Settings.rp";
say "Loaded configuration successfully";
```

## 4. Module Search Path & Resolution Order

When resolving a module name (e.g. `use String::Utils`), the Raptor interpreter searches the following locations in order:

1. **Current Working Directory (`./`)**: Direct file lookups like `String/Utils.rp`.
2. **Local Application Library (`./lib/`)**: Subdirectories under `lib/` (e.g. `lib/String/Utils.rp` or `lib/Utils.rp`).
3. **Package Manager Dependencies (`./raptor_modules/`)**: Automatically scans all cloned Git repositories located within `raptor_modules/`.
4. **Include Path Array (`@*INC` / `@INC`)**: Global module search directories configured in the runtime environment.

## 5. Raptor Package Manager (`raptor init`, `raptor get`, `raptor install`)

Raptor includes a built-in package manager inspired by Go's direct Git cloning and Node's local `raptor_modules/` directory structure.

### 5.1 Initializing a Project

Create a new package manifest in the current directory:

```powershell
raptor init my-application
```

This generates `raptor.json`, `lib/`, and `raptor_modules/`:

```json
{
  "name": "my-application",
  "version": "0.1.0",
  "description": "Raptor application",
  "dependencies": {}
}
```

### 5.2 Fetching Dependencies

```powershell
raptor get https://github.com/xyzzyapps/raptor-charm
raptor install
```
