package raptor

import (
	"testing"
)

func TestSQLiteOperations(t *testing.T) {
	in := NewInterp()

	code := `
my $db = sqlite_open(":memory:");
my $e1 = sqlite_exec($db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER);");
my $e2 = sqlite_exec($db, "INSERT INTO users (name, age) VALUES ('Alice', 30);");
my $e3 = sqlite_exec($db, "INSERT INTO users (name, age) VALUES ('Bob', 25);");

my $rows = sqlite_query($db, "SELECT id, name, age FROM users ORDER BY id ASC;");
my $first = sqlite_query_row($db, "SELECT * FROM users WHERE id = 1;");

sqlite_close($db);

[$e2<changes>, $first<name>, $first<age>];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("SQLite eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("expected 3 elements, got %+v", val)
	}

	if val.ArrayVal[0].IntVal != 1 {
		t.Errorf("expected 1 change, got %v", val.ArrayVal[0])
	}
	if val.ArrayVal[1].String() != "Alice" {
		t.Errorf("expected 'Alice', got %q", val.ArrayVal[1].String())
	}
	if val.ArrayVal[2].IntVal != 30 {
		t.Errorf("expected 30, got %v", val.ArrayVal[2])
	}
}

func TestJSONSerialization(t *testing.T) {
	in := NewInterp()

	code := `
my $data = {
    :name => "Ada Lovelace",
    :role => "Engineer",
    :skills => ["Math", "Algorithms", "Logic"],
    :active => 1,
    :score => 99.5
};

my $jsonStr = to_json($data);
my $decoded = from_json($jsonStr);

[$decoded<name>, $decoded<role>, $decoded<skills>[0], $decoded<score>];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("JSON eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 4 {
		t.Fatalf("expected 4 elements, got %+v", val)
	}

	if val.ArrayVal[0].String() != "Ada Lovelace" {
		t.Errorf("expected 'Ada Lovelace', got %q", val.ArrayVal[0].String())
	}
	if val.ArrayVal[1].String() != "Engineer" {
		t.Errorf("expected 'Engineer', got %q", val.ArrayVal[1].String())
	}
	if val.ArrayVal[2].String() != "Math" {
		t.Errorf("expected 'Math', got %q", val.ArrayVal[2].String())
	}
	if val.ArrayVal[3].FloatVal != 99.5 {
		t.Errorf("expected 99.5, got %v", val.ArrayVal[3])
	}
}
