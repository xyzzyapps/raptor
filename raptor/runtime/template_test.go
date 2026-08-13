package raptor

import (
	"strings"
	"testing"
)

func TestTemplateEchoAndCodeTags(t *testing.T) {
	tmpl := `<!DOCTYPE html>
<html>
<body>
    <h1><?= "Welcome to Raku5" ?></h1>
    <p>Sum: <?= 25 + 17 ?></p>
</body>
</html>`

	res, err := RenderTemplate(tmpl, nil)
	if err != nil {
		t.Fatalf("template render failed: %v", err)
	}

	if !strings.Contains(res, "<h1>Welcome to Raku5</h1>") {
		t.Fatalf("expected header in output, got:\n%s", res)
	}
	if !strings.Contains(res, "<p>Sum: 42</p>") {
		t.Fatalf("expected sum 42 in output, got:\n%s", res)
	}
}

func TestTemplateVariablesAndMultiDispatch(t *testing.T) {
	tmpl := `<?raku
multi sub greet(Str $name) {
    return "Hello, " ~ $name ~ "!";
}
my $user = "Antigravity";
?>
<div><?= $user.greet() ?></div>`

	res, err := RenderTemplate(tmpl, nil)
	if err != nil {
		t.Fatalf("template render failed: %v", err)
	}

	if !strings.Contains(res, "<div>Hello, Antigravity!</div>") {
		t.Fatalf("expected greeting in output, got:\n%s", res)
	}
}
