# Raptor Syntax, Variables & Dynamic Typing

Raptor uses Perl's classic sigil-based naming for crystal-clear readability without static type boilerplate.

## 1. Sigils & Variable Declaration

- `$` - Scalar values (integers, floats, strings, booleans, references, structs, closures).
- `@` - Ordered arrays / lists.
- `%` - Key-value associative hashes / maps.

```perl
# Lexical scope (my)
my $name = "Quantum Engine";
my $speed = 299792.458;
my $is_active = True;

# Array initialization
my @items = ["Alpha", "Beta", "Gamma"];
say @items[0];   # First element
say @items[-1];  # Last element (negative indexing)

# Hash initialization with string keys or pairs
my %config = {
    "host" => "127.0.0.1",
    "port" => 8080,
    "debug" => True
};
say %config{"host"}; # Hash subscripting
```

## 2. First-Class Subroutines & Closures

Subroutines and closures are first-class citizens in Raptor:

```perl
# Standard subroutine
sub greet($user) {
    return "Hello, " ~ $user ~ "!";
}

# Anonymous closure
my $double = sub ($x) { return $x * 2; };
say $double(21); # 42

# Functional list transformation with closures
my @nums = [1, 2, 3, 4, 5];
my @doubled = @nums.map(sub ($x) { return $x * 2; });
say @doubled; # [2, 4, 6, 8, 10]
```

## 3. Subroutine Signatures & Rest Parameters

Raptor supports positional parameters and slurpy rest parameters on subroutines:

```perl
sub process_list($head, @tail) {
    say "Head: " ~ $head;
    say "Tail elements: " ~ @tail.elems();
}

process_list(10, 20, 30, 40);

sub configure(%opts) {
    say "Host: " ~ %opts{"host"};
    say "Port: " ~ %opts{"port"};
}

configure({ "host" => "localhost", "port" => 8080 });
```

## 4. Uniform Function Call Syntax (UFCS)

Raptor provides Uniform Function Call Syntax (UFCS), allowing any standalone subroutine or multi-sub candidate to be called with method-call syntax on its first argument (`$invocant.subroutine(args...)`).

### 4.1 Subroutine Method Calls
```perl
sub double($x) {
    return $x * 2;
}

say 21.double(); # 42
```

### 4.2 Multiple Dispatch with UFCS
```perl
multi sub format_val($n where { $_ >= 0 }) { return "Positive: " ~ $n; }
multi sub format_val($n)                   { return "Other: " ~ $n; }

say 42.format_val();   # "Positive: 42"
say (-5).format_val(); # "Other: -5"
```

### 4.3 Functional Pipelines & Chaining
```perl
my @items = [1, 2, 3, 4, 5];
my @scaled = @items.map(sub ($x) { return $x * 10; });
say @scaled; # [10, 20, 30, 40, 50]

# Method chaining on strings
my $formatted = "  raptor runtime  ".trim().uc();
say $formatted; # "RAPTOR RUNTIME"
```
