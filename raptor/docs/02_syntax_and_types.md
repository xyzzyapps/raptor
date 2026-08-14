# Raptor Syntax, Variables & Dynamic Typing

Raptor uses Perl's classic sigil-based naming for crystal-clear readability without static type boilerplate.

---

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

# Hash initialization with colon pairs
my %config = {
    :host => "127.0.0.1",
    :port => 8080,
    :debug => True
};
say %config<host>; # Hash subscripting
```

---

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

# Pointy block lambda syntax
my @doubled = @nums.map(-> $x { $x * 2 });
```

---

## 3. Parameter Signature Destructuring

Raptor supports deep pattern matching and destructuring on function arguments:

```perl
# Array head/tail destructuring
sub process_list([$head, *@tail]) {
    say "Head: " ~ $head;
    say "Tail elements: " ~ @tail.elems;
}

process_list([10, 20, 30, 40]);

# Hash destructuring
sub authenticate(:{:$username, :$token}) {
    say "User: " ~ $username;
}

authenticate({:username => "admin", :token => "sec_123"});
```

---

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
multi sub format_val(Int $n) { return "Number: $n"; }
multi sub format_val(Str $s) { return "String: '$s'"; }

say 42.format_val();      # "Number: 42"
say "raptor".format_val(); # "String: 'raptor'"
```

### 4.3 Functional Pipelines & Chaining
```perl
my @nums = [1, 2, 3, 4, 5];
my @doubled = @nums.map(sub ($x) { $x * 10 });
say @doubled; # [10, 20, 30, 40, 50]

# Method chaining on strings and lists
my $formatted = "  raptor runtime  ".trim().uc();
say $formatted; # "RAPTOR RUNTIME"
```
