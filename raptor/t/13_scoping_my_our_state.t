plan(8);

# 1. 'my' lexical scoping in nested blocks
my $x = 10;
if True {
    my $x = 99;
    is($x, 99, "inner my variable shadows outer variable");
}
is($x, 10, "outer my variable restored after block");

# 2. 'our' package variable sharing
our $global_val = 500;
sub check_our() {
    our $global_val;
    return $global_val;
}
is(check_our(), 500, "our variable accessed across functions");

$global_val = 750;
is(check_our(), 750, "mutating our variable affects across scopes");

# 3. 'state' persistent local variables
sub counter() {
    state $count = 0;
    $count = $count + 1;
    return $count;
}

is(counter(), 1, "state counter starts at 1");
is(counter(), 2, "state counter increments to 2 on second call");
is(counter(), 3, "state counter increments to 3 on third call");

sub accumulator($val) {
    state @items = [];
    @items.push($val);
    return @items.elems();
}

is(accumulator("first"), 1, "state array accumulates elements across invocations");

done_testing();
