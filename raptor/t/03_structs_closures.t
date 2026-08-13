# Test suite: Structs, Function Pointers & Operator Overloading
plan(7);

struct Point {
    int64 $x;
    int64 $y;
    ptr $onClick;
}

# 1. Struct creation and field manipulation
my $p1 = Point.new();
$p1.x = 10;
$p1.y = 20;

is($p1.x, 10, "Point x is 10");
is($p1.y, 20, "Point y is 20");

# 2. Function pointers & closures stored on struct
$p1.onClick = sub ($scale) {
    return ($p1.x + $p1.y) * $scale;
};

my $clickRes = $p1.onClick(2);
is($clickRes, 60, "struct closure invoked via method syntax");

# 3. Custom operator overloading on structs
multi sub infix:<+>(Point $a, Point $b) {
    my $res = Point.new();
    $res.x = $a.x + $b.x;
    $res.y = $a.y + $b.y;
    return $res;
}

multi sub prefix:<->(Point $p) {
    my $res = Point.new();
    $res.x = -$p.x;
    $res.y = -$p.y;
    return $res;
}

my $p2 = Point.new();
$p2.x = 5; $p2.y = 15;

my $sum = $p1 + $p2;
is($sum.x, 15, "overloaded + sum x");
is($sum.y, 35, "overloaded + sum y");

my $neg = -$p2;
is($neg.x, -5, "overloaded prefix - neg x");
is($neg.y, -15, "overloaded prefix - neg y");

done_testing();
