# Raptor Literate Programming Manual (PodLit)

## Overview

Raptor features a built-in **Literate Programming** subsystem based on an extended POD (Plain Old Documentation) format named **PodLit**.

Literate Programming, pioneered by Donald Knuth, treats software as an explanatory work of literature directed to humans, from which executable machine code is generated.

PodLit supports three fundamental operations:
1. **Weave**: Generates GitHub-Flavored Markdown (`.md`) formatted for humans from prose and code.
2. **Tangle**: Extracts source code files (`.rp`, `.raptor`, `.c`) by recursively assembling named code chunks (`<<chunk-name>>`).
3. **Mangle**: Applies code transformations and macro pipeline filters (`:mangle(...)`).
4. **Direct Literate Execution**: Directly executes `.pod` literate documents via in-memory tangling.

---

## 1. PodLit Directives

| Directive | Description |
| :--- | :--- |
| `=pod` | Begins a POD documentation block |
| `=head1` .. `=head6 <title>` | Markdown-compatible section headings |
| `=item <text>` | Bulleted list item |
| `=chunk <name> [:file "path"] [:mangle(...)]` | Declares a named code chunk |
| `=end chunk` | Ends a code chunk declaration |
| `=cut` | Ends POD block and returns to raw code/prose |

Inline formatting:
- `B<bold>` -> `**bold**`
- `I<italic>` -> `*italic*`
- `C<code>` -> `` `code` ``
- `L<url>` -> `[url](url)`

---

## 2. Tangling & Chunk Expansion (`<<chunk>>`)

Chunks can reference other chunks by name:

```perl
=chunk <vector-type>
struct Vec2 {
    num32 $x;
    num32 $y;
}
=end chunk

=chunk <main> :file "bin/app.rp"
<<vector-type>>

say "Vector initialized";
=end chunk
```

When tangled, `<<vector-type>>` is recursively replaced with its code body while preserving the caller's indentation level.

---

## 3. Mangle Transformations

You can attach filters to any chunk using `:mangle(...)`:

- `:mangle(indent(4))`: Indents all lines by 4 spaces.
- `:mangle(strip_comments)`: Removes comments for production builds.
- `:mangle(prefix("lib_"))`: Prepends a prefix string.

---

## 4. Stitching (Reverse-Tangling & Round-Trip Sync)

When developers edit generated source files (`.rp`, `.raptor`, `.c`) in their IDE or text editor, **Stitch** synchronizes those changes back into the original `.pod` literate document:

- Preserves 100% of human narrative prose, section headings, and formatting.
- Replaces chunk bodies inside `=chunk <name>` ... `=end chunk` with the edited implementations.

```powershell
# Stitch a single edited file back into the POD:
raptor stitch examples/literate_game.pod src/lib/LiterateTypes.rp

# Stitch an entire tangled directory tree:
raptor stitch examples/literate_game.pod src/ -o docs/updated_game.pod
```

---

## 5. CLI Commands

```powershell
# 1. Weave documentation to Markdown
raptor weave examples/literate_game.pod -o docs/literate_game.md

# 2. Tangle source files to disk
raptor tangle examples/literate_game.pod -o ./src/

# 3. Stitch modified source files back into the literate document
raptor stitch examples/literate_game.pod ./src/lib/LiterateTypes.rp

# 4. Direct Literate Execution
raptor examples/literate_game.pod
raptor run examples/literate_game.pod
```

---

## 6. Runtime Builtins

Raptor programs can also dynamically weave, tangle, and stitch documents programmatically:

```raptor
my $pod = slurp("document.pod");

# Weave to Markdown
my $markdown = pod_weave($pod);

# Tangle to file hash (%files{"path"} => code)
my %files = pod_tangle($pod);
for %files.keys -> $filename {
    say "Extracted: " ~ $filename ~ " (" ~ %files{$filename}.chars ~ " chars)";
}

# Stitch modified files back into POD
my %modified = {
    :lib_Math_Add => "sub add($x, $y) { return ($x + $y) * 10; }"
};
my $updatedPod = pod_stitch($pod, %modified);
spurt("document.pod", $updatedPod);
```
