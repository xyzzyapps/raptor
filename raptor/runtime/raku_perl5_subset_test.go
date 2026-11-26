package raptor

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func readTestFileBytes(p string) ([]byte, error) { return os.ReadFile(p) }

func runRaptorSnippet(t *testing.T, src string) string {
	t.Helper()
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatalf("ParseProgram failed: %v\nSource:\n%s", err, src)
	}
	in := NewInterp()
	var buf bytes.Buffer
	in.Stdout = &buf
	in.Stderr = &buf
	in.ResolveProgram(prog, true)
	if _, err := in.EvalProgram(prog, in.GlobalEnv); err != nil {
		t.Fatalf("Eval failed: %v\nOutput:\n%s\nSource:\n%s", err, buf.String(), src)
	}
	return strings.TrimSpace(buf.String())
}

func TestGrammarActionsMakeAndNamedCaptureVars(t *testing.T) {
	out := runRaptorSnippet(t, `
grammar CalcGram {
    rule TOP { <term> [ $<op>=['+' | '-'] <term> ]* }
    token term { \d+ }
}

my %actions = {
    "term" => sub ($m) {
        make(+$m<Str>);
    },
    "TOP" => sub ($m) {
        my $terms = $m<term>;
        if type($terms) eq "Hash" {
            make($terms<made>);
            return;
        }
        my $acc = $terms[0]<made>;
        my $ops = $m<op>;
        if type($ops) eq "Hash" {
            $ops = [$ops];
        }
        for 0 ..^ elems($ops) -> $i {
            if $ops[$i]<Str> eq "+" {
                $acc += $terms[$i + 1]<made>;
            } else {
                $acc -= $terms[$i + 1]<made>;
            }
        }
        make($acc);
    }
};

my $m = CalcGram.parse("10 + 25 - 5", %actions);
say $m<made>;
say $m.ast;

grammar PairGram {
    rule TOP { $<key>=<ident> ':' :i $<val>=['true' | 'false'] }
}
PairGram.parse("debug: TRUE");
say "$<key><Str>=$<val><Str>";
`)
	want := "30\n30\ndebug=TRUE"
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestRakuAndPerl5SubsetFeatures(t *testing.T) {
	out := runRaptorSnippet(t, `
# 1. Prefix & postfix ++ / -- and hash auto-vivification
my $x = 5;
say $x++;
say ++$x;
say $x--;
say $x;

my %freq;
for qw(perl raku raptor raku raptor raptor) -> $w {
    %freq{$w}++;
}
say "%freq<perl>,%freq<raku>,%freq<raptor>";

# 2. List destructuring and swap
my ($a, $b, @rest) = (10, 20, 30, 40, 50);
($a, $b) = ($b, $a);
say "$a,$b,{join('-', @rest)}";

# 3. Default parameters, slurpy *@rest, and Perl 5 @_
sub greet($who = "World", *@extra) {
    return "Hello $who (" ~ join(",", @extra) ~ ")";
}
say greet();
say greet("Raku", "fast", "peg");

sub sum_perl5 {
    my $s = 0;
    foreach my $v (@_) {
        $s += $v;
    }
    return $s;
}
say sum_perl5(1, 2, 3, 4, 5);

# 4. C-style for, exclusive ranges (..^, ^N), multi-var pointy -> $k, $v, angle word-list
my $csum = 0;
for (my $i = 0; $i < 5; $i++) {
    $csum += $i;
}
say $csum;
say join(",", ^5);
say join(",", 2 ..^ 6);
say join(",", 1 ^.. 4);
say join(",", 1 ^..^ 5);

my @pairs = <alpha 1 beta 2 gamma 3>;
my @formatted;
for @pairs -> $k, $v {
    push(@formatted, "$k=$v");
}
say join(";", @formatted);

# 5. try / catch + $! + eval
my $bad = try { die("boom"); 99 };
say defined($bad) ?? "defined" !! "undef";
say contains($!, "boom");
say eval("6 * 7");

# 6. Regex captures ($1, $2), subst, words, flip, pack/unpack, sum
my $line = "user=alice id=42";
if $line =~ /user=(\w+)\s+id=(\d+)/ {
    say "$1:$2";
}
say subst("foo   bar   baz", "\\s+", "-");
say flip("stressed");
say reverse("desserts");
say sum([10, 20, 30, 40]);

my $packed = pack("n C2", 0x1234, 65, 66);
my @unp = unpack("n C2", $packed);
say "{@unp[0]},{@unp[1]},{@unp[2]}";
`)
	expectedLines := []string{
		"5",
		"7",
		"7",
		"6",
		"1,2,3",
		"20,10,30-40-50",
		"Hello World ()",
		"Hello Raku (fast,peg)",
		"15",
		"10",
		"0,1,2,3,4",
		"2,3,4,5",
		"2,3,4",
		"2,3,4",
		"alpha=1;beta=2;gamma=3",
		"undef",
		"True",
		"42",
		"alice:42",
		"foo-bar-baz",
		"desserts",
		"stressed",
		"100",
		"4660,65,66",
	}
	want := strings.Join(expectedLines, "\n")
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}
