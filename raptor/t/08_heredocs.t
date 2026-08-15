plan(4);

# 1. Unquoted heredoc with interpolation
my $name = "Raptor";
my $doc1 = <<EOF;
Hello $name!
Welcome to Heredocs.
EOF

like($doc1, "Hello Raptor!", "interpolated heredoc contains variable");
like($doc1, "Welcome to Heredocs.", "interpolated heredoc second line");

# 2. Single-quoted raw heredoc
my $doc2 = <<'EOF';
Hello $name!
No interpolation here.
EOF

is($doc2, "Hello \$name!\nNo interpolation here.\n", "single-quoted heredoc does not interpolate");

# 3. Indented heredoc (<<~)
my $doc3 = <<~EOF;
    Line 1
    Line 2
    EOF

is($doc3, "Line 1\nLine 2\n", "indented heredoc strips common leading indentation");

done_testing();
