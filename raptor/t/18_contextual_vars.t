plan(9);

# 1. $*RAPTOR object
is($*RAPTOR.version, "1.0.0", "\$*RAPTOR.version is 1.0.0");
is($*RAPTOR.name, "Raptor", "\$*RAPTOR.name is Raptor");

# 2. $*KERNEL object — name is GOOS (windows / linux / darwin / ...)
is($*KERNEL.name, $*OS, "\$*KERNEL.name matches OS");
ok($*KERNEL.arch ne "", "\$*KERNEL.arch is set");

# 3. $*PID and $$
ok($*PID > 0, "\$*PID is positive integer");
is($$, $*PID, "\$\$ equals \$*PID");

# 4. %*ENV environment hash
is(ref(\%*ENV), "HASH", "%*ENV is a valid hash reference");

# 5. $*PROGRAM / $*PROGRAM-NAME
ok($*PROGRAM ne "", "\$*PROGRAM contains program name");

# 6. Exit code variable $?
is($?, 0, "\$? initialized to 0");

done_testing();
