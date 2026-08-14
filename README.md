# Raptor

[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](raptor/t/)
[![License: Artistic-2.0](https://img.shields.io/badge/License-Artistic_2.0-0298c3.svg)](https://opensource.org/licenses/Artistic-2.0)

Raptor is engineered specifically for the Post-LLM paradigm:

1. **High Token Density & Brief Syntax**: By adopting the concise syntax of Perl 5 and modern Raku (variables using `$` `@` `%` sigils, canonical `Nil`, built-in operators) while completely omitting `class`, `has`, and `is` boilerplate, Raptor maximizes LLM context window efficiency and reduces model hallucinations.
2. **Verification-First by Default**: LLMs generate code rapidly, but need tight, verifiable guardrails. Raptor builds formal Design-by-Contract (`pre`, `post`, `invariant`), inline testing (`TEST`, `subtest`), and randomized property-based quickcheck fuzzing (`property`) directly into the core grammar.
3. **Continuous Invariant Checking on Every Assignment**: Unlike static types checked once at compile time, every variable in Raptor can hold a dynamic `where` predicate that is rigorously evaluated upon **every mutation and assignment**. If an LLM or runtime mutation produces an invalid state, it is caught immediately at the point of failure.
4. **Uniform Function Call Syntax (UFCS)**: Functional method chaining (`$val.func().transform()`) allows LLMs to construct clean data transformation pipelines without wrapping primitives in artificial classes.
5. **C-ABI Struct Memory & Closures**: Contiguous C-compatible memory layout records (`struct Vector2 { num64 $x; num64 $y; }`) with first-class closure fields provide native C performance without OOP overhead.
6. **Literate Programming with PodLit**: Knuth-style chunk weaving, source tangling, and reverse-stitching allow LLMs to maintain human-readable architectural specifications and executable source code side-by-side.

## Academic & Theoretical Foundations

Raptor's predicate type, continuous contract, and literate programming systems are grounded in seminal computer science research:

- **Robert Bruce Findler (PhD Thesis, Rice University, 2002)**: *Behavioral Software Contracts* (supervised by Matthias Felleisen). Established the mathematical and runtime foundations of higher-order dynamic contracts, continuous invariant monitoring on mutable reference cells, and assignment-level contract enforcement.
- *Predicate Dispatching: A Unified Theory of Dispatch* (with Craig Kaplan & Craig Chambers). Introduced the formal model for evaluating arbitrary user-defined boolean predicates over variables and parameter values to drive runtime dispatch and enforce state invariants.
- **Patrick Maxim Rondon (PhD Thesis, UC San Diego, 2012)**: *Liquid Types* (supervised by Ranjit Jhala, built on Frank Pfenning's 1991 *Refinement Types for ML*). Refinement types via predicate subtyping where standard base types are enriched with logical predicates evaluated against values and assignments.
- **Johan Hidding (Netherlands eScience Center, 2023)**: *Entangled, a Bidirectional System for Sustainable Literate Programming*, 2023 IEEE 19th International Conference on e-Science (e-Science), pp. 1-10, [DOI: 10.1109/e-Science58273.2023.10254816](https://doi.org/10.1109/e-Science58273.2023.10254816). Formulated the model and grammar for bidirectional, round-trip literate programming where external code modifications are synchronized and reverse-stitched back into literate narrative documents without disturbing narrative prose.

## Core Subprojects

### 1. [Raptor (`raptor/`)](raptor/README.md)
Raptor is the core execution platform and post-LLM language runtime:
- **Core Runtime**: Pure dynamic typing (`$scalar`, `@array`, `%hash`), Uniform Function Call Syntax (UFCS), C-ABI `struct` records, function pointer closures, and Raku-style dynamic `subset` refinements with `where` predicate dispatching and continuous assignment validation.
- **Charmbracelet TUI Engine**: Lip Gloss ANSI 24-bit TrueColor styling (`tui_style`), framed viewports (`tui_box`), data tables (`tui_table`), progress bars (`tui_progress`), and Bubble Tea state-machine event loops (`tui_app_run`).
- **PodLit Literate Programming**: Knuth-style literate programming (`raptor weave`, `raptor tangle`, `raptor stitch`).
- **Verification & Testing**: Full TAP v13 test producer, `raptor test` harness (like `prove`), Design-by-Contract (`pre`, `post`, `invariant`), and QuickCheck fuzzing.
- **WebAssembly (Wasm) Go Tour**: 100% client-side in-browser playground (`web/raptor.wasm`, `raptor serve --port 8080`) with Canvas 2D graphics, WebAudio sound synthesizer, DOM manipulation, and interactive Go Tour lessons.
- **RaptorHP Template Server (`raptorhp.exe`)**: PHP-style embedded templating engine and development HTTP server with superglobals (`%_GET`, `%_POST`, `%_SERVER`).
- **Package Manager**: Git-based package management (`raptor init`, `raptor get <repo>`, `raptor install`) cloning dependencies directly into `./raptor_modules/`.
- **Single-Binary Packager**: Packages scripts into standalone `.exe` binaries (`raptor pack`).

### 2. [MoarVM-Go (`moarvm-go/`)](moarvm-go/README.md)
A high-performance Go host and FFI binding for **MoarVM** (64-bit 6Model JIT virtual machine via `moar.dll`).
- **CompUnit v7 Emitter**: Serializes valid binary bytecode with Serialization Contexts (SC), frame tables, and register descriptors.
- **Metamodel (6Model)**: Pluggable object representations (`P6opaque`, `MVMArray`, `MVMHash`).
- **Declarative Raku Grammars**: Pure-Go pattern matching and Pratt operator precedence parsing engine.
- **Tcl Reference Frontend**: Strict Tcl language implementation with C and Go FFI.

## Quickstart Feature Tutorial

### 1. Readability, Brief Syntax & Canonical `Nil`
Raptor combines the concise syntax of Perl 5 with Raku's modern semantics. Variables use standard sigils (`$`, `@`, `%`) with pure dynamic typing and canonical `Nil`:

```raku
# Variables and canonical Nil
my $name = "Raptor";
my @primes = [2, 3, 5, 7, 11];
my %config = { "host" => "localhost", "port" => 8080 };

my $default_val = Nil // "Fallback Value";
say "Server: " ~ $name ~ " running on " ~ %config{"host"} ~ ":" ~ %config{"port"};
say "Defined-or check: " ~ $default_val;
```

### 2. Uniform Function Call Syntax (UFCS)
Any subroutine or multiple-dispatch candidate can be chained using method syntax on its first argument without boilerplate wrapper classes:

```raku
sub format_id($n) { return "ID-#" ~ $n; }
sub tag($text, $t) { return "<" ~ $t ~ ">" ~ $text ~ "</" ~ $t ~ ">"; }

# Method chaining via UFCS
say 1001.format_id();                           # ID-#1001
say "welcome".uc().tag("h1");                  # <h1>WELCOME</h1>

my @items = [10, 20, 30];
say @items.elems();                             # 3
```

### 3. Predicate Types & Continuous Invariant Enforcement
Drawing from the formal foundations of **Predicate Subtyping, Refinement Types (Liquid Types), and Design-by-Contract**, Raptor enforces dynamic predicates not only on parameter dispatch but continuously across **every assignment**:

```raku
# Define dynamic refinement predicate types
subset Positive where { $_ > 0 };
subset PortNum  where { $_ >= 1 && $_ <= 65535 };

# Variable invariants evaluated on declaration AND every subsequent mutation
my Positive $balance = 100;
$balance = 250;      # OK
# $balance = -10;    # Throws: type check failed: dynamic constraint violated for subset Positive

my $port where { $_ >= 1 && $_ <= 65535 } = 8080;
$port = 9000;        # OK
# $port = 99999;     # Throws: type check failed: where constraint violated for variable $port

# Predicate-based Multiple Dispatch & Pattern Recursion
multi sub fib($n where { $n <= 1 }) { return $n; }
multi sub fib($n)                   { return fib($n - 1) + fib($n - 2); }

say "fib(8) = " ~ fib(8);
```

### 4. C-ABI Contiguous Structs & Closure Fields
Raptor provides zero-overhead C-compatible contiguous memory records with first-class function pointer fields (closures) and custom operator overloading:

```raku
struct Vector2 {
    num64 $x;
    num64 $y;
}

# Operator overloading on C-ABI structs
multi sub infix:<+>(Vector2 $a, Vector2 $b) {
    my $r = Vector2.new();
    $r.x = $a.x + $b.x;
    $r.y = $a.y + $b.y;
    return $r;
}

struct Button {
    int32 $id;
    ptr $onClick;
}

my $v1 = Vector2.new(); $v1.x = 10.0; $v1.y = 20.0;
my $v2 = Vector2.new(); $v2.x = 5.0;  $v2.y = 15.0;
my $sum = $v1 + $v2;
say "Vector Sum: (" ~ $sum.x ~ ", " ~ $sum.y ~ ")";

my $btn = Button.new();
$btn.id = 42;
$btn.onClick = sub ($env) { say "Button [ID:" ~ $btn.id ~ "] clicked for: " ~ $env; };
$btn.onClick("Production");
```

### 5. Verification: Design-by-Contract & Property Fuzzing
Raptor integrates formal verification directly into the language with pre-conditions, post-conditions, invariants, and randomized property-based quickcheck fuzzing:

```raku
sub safe_divide($a, $b) {
    pre({ $b != 0 });
    post({ $_ >= 0 });
    return $a div $b;
}

# Inline subtest verification
subtest "Arithmetic invariants", sub () {
    plan(2);
    is(safe_divide(10, 2), 5, "valid division");
    is(10 + 20, 30, "sum invariant");
};

# Randomized property-based testing (100 trials)
property "addition commutativity", sub ($x, $y) { return ($x + $y) == ($y + $x); };
property "multiplication by zero", sub ($x, $y) { return ($x * 0) == 0; };
```

### 6. Charmbracelet TUI & Terminal Styling Engine
Build sophisticated terminal user interfaces using built-in Lip Gloss TrueColor styling, boxed viewports, tables, progress indicators, and Bubble Tea state machines:

```raku
# Lip Gloss 24-bit TrueColor ANSI styling
my $title = tui_style("RAPTOR MONITOR", {:fg => "#00ADD8", :bold => True});

# Framed viewports & status boxes
my $box = tui_box("Status: Active\nUptime: 99.99%", {:title => "RAPTOR MONITOR", :border => "rounded"});
say $box;

# Dynamic data tables
my @headers = ["Service", "Status", "Latency"];
my @rows = [
    ["Auth Gateway", "200 OK", "1.2ms"],
    ["Database Core", "200 OK", "0.8ms"]
];
say tui_table(@headers, @rows);

# Progress indicators
say tui_progress(0.75, {:width => 30});
```

### 7. Literate Programming with PodLit
Write literate code incorporating Knuth-style chunk weaving, source tangling, and bidirectional reverse-stitching:

```raku
my $podSource = q[=begin pod
=head1 Literate Math Engine

<<add-implementation>>
=begin code
sub add($a, $b) { return $a + $b; }
=end code
=end pod];

# 1. Weave to human-readable Markdown documentation
my $docs = pod_weave($podSource);

# 2. Tangle directly into compiled source files
my %files = pod_tangle($podSource);

# 3. Reverse-stitch edited code back into documentation
my %updates = { "add-implementation" => "sub add($a, $b) { return $a + $b; }" };
my $updatedPod = pod_stitch($podSource, %updates);
```

## Building & Testing from Source

### Linux (WSL / Native)
```bash
# 1. Install prerequisites (Alpine example: apk add go gcc musl-dev sqlite-dev portaudio-dev git make)
# Debian/Ubuntu: sudo apt install -y golang gcc libsqlite3-dev libportaudio2 portaudio19-dev git make

# 2. Build binaries
cd raptor
go build -o bin/raptor ./cmd/raptor
go build -o bin/raptorhp ./cmd/raptorhp

# 3. Run unit tests & TAP test harness
go test -v ./...
./bin/raptor test t/
./bin/raptor run examples/demo_showcase.rp
```

### Windows (PowerShell)
```powershell
cd raptor
go build -o bin/raptor.exe ./cmd/raptor
go build -o bin/raptorhp.exe ./cmd/raptorhp
go test ./...
.\bin\raptor.exe test t\
```

## License

Licensed under the **Artistic License 2.0**.

## Agent Skills (`.agents/skills/`)
- **[moarvm-language-development](.agents/skills/moarvm-language-development/SKILL.md)**: Guide for agents on building new compiled languages targeting MoarVM.
- **[raptor-language-guide](.agents/skills/raptor-language-guide/SKILL.md)**: Comprehensive guide for agents on the Raptor runtime, C FFI, Wasm, RaptorHP, and PodLit.
- **[podlit-literate-programming](.agents/skills/podlit-literate-programming/SKILL.md)**: Guide for agents on writing, weaving, tangling, mangling, and reverse-stitching literate programs in PodLit.
