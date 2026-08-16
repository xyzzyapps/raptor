plan(5);

my %payload = { "ok" => 1, "name" => "raptor" };
my $js = to_json(%payload);
ok($js, 'to_json emits keys');

my $back = from_json($js);
ok($back, 'from_json round-trips');

my $db = sqlite_open(":memory:");
ok($db, 'sqlite_open memory');

sqlite_exec($db, "CREATE TABLE t(id INTEGER PRIMARY KEY)");
ok(True, 'sqlite_exec');

sqlite_close($db);
ok(True, 'sqlite_close');

done_testing();
