plan(7);

# 1. Postfix if modifier
my $val = 10;
my $flag = True;
$val = 42 if $flag;
is($val, 42, "postfix if executes when true");

$val = 99 if False;
is($val, 42, "postfix if skips when false");

# 2. Postfix unless modifier
my $status = "OK";
$status = "ERROR" unless True;
is($status, "OK", "postfix unless skips when true");

$status = "RETRY" unless False;
is($status, "RETRY", "postfix unless executes when false");

# 3. Postfix while and until modifiers
my $counter = 0;
$counter = $counter + 1 while $counter < 5;
is($counter, 5, "postfix while loop executes until condition false");

my $down = 5;
$down = $down - 1 until $down <= 0;
is($down, 0, "postfix until loop executes until condition true");

# 4. Postfix for modifier
my @items = [10, 20, 30];
my $sum = 0;
$sum = $sum + $_ for @items;
is($sum, 60, "postfix for modifier iterates with \$_");

done_testing();
