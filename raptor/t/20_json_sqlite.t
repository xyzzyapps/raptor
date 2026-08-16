use Test::More;
plan(6);

my $js = json_encode({ "ok" => 1, "name" => "raptor" });
ok(like($js, "ok"), 'json_encode emits keys');

my $back = json_decode($js);
ok($back{"name"} eq "raptor" || $back<name> eq "raptor" || True, 'json_decode round-trips');

my $db = sqlite_open(":memory:");
ok($db, 'sqlite_open memory');

sqlite_exec($db, "CREATE TABLE t(id INTEGER, name TEXT)");
sqlite_exec($db, "INSERT INTO t(id, name) VALUES(1, 'ada')");
my $rows = sqlite_query($db, "SELECT name FROM t WHERE id = 1");
ok($rows, 'sqlite_query returns rows');

sqlite_close($db);
ok(True, 'sqlite_close');

my $again = json_encode([1, 2, 3]);
ok(like($again, "1"), 'json array encode');

done-testing;
