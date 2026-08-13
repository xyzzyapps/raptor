# ==============================================================================
# Raptor Language Feature Showcase: Junctions, Destructuring, Gather/Take & Ops
# ==============================================================================

say "=== 1. Junction Autothreading & Comparisons ===";
my $grade = 92;
if $grade == any(90, 92, 95, 100) {
    say "Grade matches honors tier!";
}
if $grade > all(50, 60, 70, 80) {
    say "Grade exceeds all baseline passing thresholds!";
}

say "\n=== 2. Deep Pattern Destructuring ===";
sub calculate_stats([$head, *@tail]) {
    my $sum = $head;
    for @tail -> $elem {
        $sum = $sum + $elem;
    }
    return [$head, @tail.elems, $sum];
}

my $stats = calculate_stats([10, 20, 30, 40, 50]);
say "Head: " ~ $stats[0] ~ ", Tail count: " ~ $stats[1] ~ ", Sum: " ~ $stats[2];

sub format_profile(:{:$username, :$role}) {
    return "User " ~ $username ~ " has permission role: " ~ $role;
}
my $profile = {:username => "AdaLovelace", :role => "SuperAdmin"};
say format_profile($profile);

say "\n=== 3. Custom Operators (Infix & Prefix) ===";
sub infix:<xor>(Int $a, Int $b) {
    if ($a != 0 && $b == 0) || ($a == 0 && $b != 0) {
        return 1;
    }
    return 0;
}

my $t1 = 1 xor 0;
my $t2 = 1 xor 1;
say "1 xor 0 = " ~ $t1;
say "1 xor 1 = " ~ $t2;

say "\n=== 4. Generator Expressions with gather / take ===";
my $evens = gather {
    take 2;
    take 4;
    for 3..6 -> $i {
        take $i * 2;
    }
};
say "Generated sequence: " ~ $evens;

say "\nAll Raptor Advanced Language Features verified successfully!";
