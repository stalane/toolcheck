package registry

import "testing"

func TestRegistryNamesUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, tool := range All() {
		if tool.Name == "" {
			t.Fatal("empty tool name")
		}
		if seen[tool.Name] {
			t.Fatalf("duplicate tool %q", tool.Name)
		}
		seen[tool.Name] = true
		if tool.Source == "" {
			t.Fatalf("%s: empty source", tool.Name)
		}
		if tool.Synthetic == nil && len(tool.Commands) == 0 {
			t.Fatalf("%s: no command and no synthetic detector", tool.Name)
		}
	}
}

func TestUpdateKindsAreKnown(t *testing.T) {
	known := map[UpdateKind]bool{
		KindNPM: true, KindUV: true, KindBun: true, KindDeno: true, KindComposer: true, KindRustup: true,
	}
	for _, tool := range All() {
		if tool.Update != nil && !known[tool.Update.Kind] {
			t.Fatalf("%s: unknown update kind %q", tool.Name, tool.Update.Kind)
		}
	}
}

func TestByName(t *testing.T) {
	if tool, ok := ByName("pip3"); !ok || tool.Name != "pip" {
		t.Fatalf("ByName(pip3) = %+v, %v", tool, ok)
	}
	if _, ok := ByName("definitely-not-a-tool"); ok {
		t.Fatal("unexpected match")
	}
}
