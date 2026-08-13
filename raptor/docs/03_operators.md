# Raptor Operators Reference

Raptor provides a complete suite of expressive operators from Perl 5 and Raku.

---

## 1. Operator Table

| Operator | Type | Description | Example |
| :--- | :--- | :--- | :--- |
| `//` | Infix | Defined-or | `my $v = $arg // "default";` |
| `//=` | Infix | Defined-or assign | `$cache //= compute();` |
| `**` | Infix | Exponentiation | `my $p = 2 ** 8; # 256` |
| `?? !!` | Ternary | Conditional expr | `$age >= 18 ?? "Adult" !! "Minor"` |
| `+&` | Infix | Numeric bitwise AND | `0xFF +& 0x0F # 15` |
| `+\|` | Infix | Numeric bitwise OR | `0x0F +\| 0xF0 # 255` |
| `+^` | Infix | Numeric bitwise XOR | `0xFF +^ 0x0F # 240` |
| `+<`, `+>` | Infix | Bitwise shift left/right | `1 +< 4 # 16` |
| `x` | Infix | String repetition | `"Na " x 4 # "Na Na Na Na "` |
| `xx` | Infix | List repetition | `[1, 2] xx 3 # [1, 2, 1, 2, 1, 2]` |
| `div` | Infix | Integer division | `10 div 3 # 3` |
| `mod` | Infix | Modulo | `10 mod 3 # 1` |
| `%%` | Infix | Divisibility test | `100 %% 2 # True` |
| `min`, `max` | Infix | Minimum / Maximum | `10 min 20 # 10` |
| `=~`, `!~` | Infix | Regex match / non-match | `$email =~ /^[^@]+@[^@]+$/` |
| `~~` | Infix | Smart matching | `$val ~~ Even # True` |
| `∈`, `∉` | Infix | Set membership | `42 ∈ [10, 42, 99] # True` |
| `∩`, `∪` | Infix | Set intersection / union | `@a ∩ @b` |

---

## 2. File Test Operators

Raptor supports standard file test operators:
- `-e $path`: True if file or directory exists.
- `-f $path`: True if path is a plain file.
- `-d $path`: True if path is a directory.
- `-s $path`: Returns file size in bytes (0 if empty/non-existent).
- `-r $path`: True if readable.
- `-w $path`: True if writable.

```perl
if -f "config.json" {
    my $content = slurp("config.json");
}
```

---

## 3. Quantum Autothreading Junctions

Junctions (`all`, `any`, `one`, `none`) autothread across comparison and logical operators:

```perl
my $x = 15;
if $x == any(10, 15, 20) {
    say "Match found!";
}

if all(10, 20, 30) > 5 {
    say "All numbers exceed 5!";
}
```
