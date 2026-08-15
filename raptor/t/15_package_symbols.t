plan(7);

# 1. Package block declaration
package MathEngine {
    sub add($a, $b) {
        return $a + $b;
    }
    our $version = "2.0";
}

is(MathEngine::add(10, 20), 30, "qualified sub call MathEngine::add succeeds");
is($MathEngine::version, "2.0", "qualified package variable \$MathEngine::version is 2.0");

# 2. Package symbol table reflection
my $symbols = package_symbols("MathEngine");
is(ref($symbols), "HASH", "package_symbols returns hash");
is(package_get("MathEngine", "version"), "2.0", "package_get retrieves package variable");

# 3. Dynamic package symbol insertion (metaprogramming)
package_set("MathEngine", "mult_by_ten", sub ($x) {
    return $x * 10;
});

is(MathEngine::mult_by_ten(5), 50, "dynamically installed package sub callable via MathEngine::mult_by_ten");

# 4. Direct %Package:: stash access
my %stash = %MathEngine::;
is(%stash{"version"}, "2.0", "direct stash %MathEngine:: lookup contains version");

# 5. Dynamic package symbol deletion
package_delete("MathEngine", "version");
is(package_get("MathEngine", "version"), Nil, "package_delete successfully removed symbol");

done_testing();
