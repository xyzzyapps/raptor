# Test suite: Charmbracelet TUI, Lip Gloss Styling & Markdown
plan(5);

# 1. Lip Gloss styling
my $styled = tui_style("Hello Charm", {:bold => True, :fg => "#38bdf8"});
ok($styled ne "", "tui_style produces non-empty styled text");

# 2. Box rendering
my $box = tui_box("Status: OK", {:title => "Health", :border => "rounded"});
ok(index($box, "Health") >= 0, "tui_box contains title");
ok(index($box, "Status: OK") >= 0, "tui_box contains content");

# 3. Table rendering
my @headers = ["Service", "Port"];
my @rows = [["HTTP", "80"], ["HTTPS", "443"]];
my $table = tui_table(@headers, @rows);
ok(index($table, "HTTP") >= 0, "tui_table contains HTTP header/row");

# 4. Progress bar rendering
my $pbar = tui_progress(0.5, {:width => 20});
ok(index($pbar, "50%") >= 0, "tui_progress displays 50%");

done_testing();
