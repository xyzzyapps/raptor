# Raptor Language & Runtime - Introduction

Raptor is a high-performance, strictly non-OO procedural execution platform and language runtime designed as the **pure dynamic Perl 5 subset of Raku**. It targets **MoarVM** (64-bit JIT, 6model virtual machine), native OS dynamic libraries via C FFI, TCP/UDP sockets, real-time audio synthesis (PortAudio), SQLite, Charmbracelet terminal styling, dynamic refinement types with `subset`, and standalone binary packaging (`raptor pack`).


## 1. Core Philosophy

1. **No Object-Oriented Keywords**: Raptor rejects class hierarchies, inheritance, and OOP boilerplate (`class`, `has`, and `is` keywords are not present).
2. **Pure Dynamic Variables**: All variables (`$scalar`, `@array`, `%hash`) are dynamically typed like Perl 5.
3. **C-ABI Struct Records**: Data records are defined using `struct` with natural C-ABI memory layouts, supporting function pointer fields and closures.
4. **Dynamic Refinement Types & Predicate Dispatching**: Subsets are defined with `where` boolean predicates (`subset Positive where { $_ > 0 }`) and checked strictly at runtime.
5. **Uniform Function Call Syntax (UFCS)**: Any subroutine `fn($obj, $arg)` can be invoked naturally as `$obj.fn($arg)`.


## 2. Quick Tour

```perl
# Dynamic variables
my $name = "Raptor";
my @nums = [1, 2, 3, 4, 5];
my %user = {:name => "Alice", :role => "Admin"};

# Raku-style dynamic refinement types
subset Positive where { $_ > 0 };
my Positive $score = 100;

# Multiple dispatch with predicate dispatching
multi sub solve($n where { $n <= 1 }) { return 1; }
multi sub solve($n where { $n > 1 })  { return $n * solve($n - 1); }

say "Factorial of 5: " ~ solve(5);
```


## 3. Command Line Interface

```powershell
# Run a script (.rp or .raptor)
raptor run app.rp

# Launch the WebAssembly In-Browser Playground & REPL
raptor serve --port 8080

# Run the PHP-Style Template Server (RaptorHP)
raptorhp -S localhost:8000

# Weave, Tangle, and Stitch Literate POD documents
raptor weave doc.pod -o doc.md
raptor tangle doc.pod -o src/
raptor stitch doc.pod src/lib/Types.rp

# Run the TAP test harness (like prove)
raptor test t/

# Read manual documentation in terminal
raptor doc operators

# Package into a standalone self-contained .exe
raptor pack app.rp -o app.exe
```
