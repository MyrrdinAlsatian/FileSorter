package renamer

import "testing"

func TestParsePatternAndValidate(t *testing.T) {
	pattern, err := ParsePattern("{date}_{seq:4}.{ext}")
	if err != nil {
		t.Fatal(err)
	}
	if err := pattern.Validate(); err != nil {
		t.Fatal(err)
	}
	if !pattern.HasVariable("date") || !pattern.HasVariable("seq") || pattern.HasVariable("unknown") {
		t.Fatal("pattern variables were parsed incorrectly")
	}
}

func TestNewRenamerRejectsUnknownVariable(t *testing.T) {
	if _, err := NewRenamer("{does_not_exist}"); err == nil {
		t.Fatal("NewRenamer accepted an unknown variable")
	}
}

func TestConflictResolverIncrement(t *testing.T) {
	resolver := NewConflictResolver(StrategyIncrement)
	first := resolver.Resolve("photos", "image", ".jpg", "")
	second := resolver.Resolve("photos", "image", ".jpg", "")
	if first.Name != "image.jpg" || first.Conflict {
		t.Fatalf("first resolution = %#v", first)
	}
	if second.Name != "image_0002.jpg" || !second.Conflict {
		t.Fatalf("second resolution = %#v", second)
	}
}
