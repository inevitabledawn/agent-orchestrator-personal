package main

import "testing"

func TestUniqueEnumNameKeepsCollidingWireValuesDistinct(t *testing.T) {
	used := map[string]bool{}
	first := uniqueEnumName("openai/form", used)
	second := uniqueEnumName("openaiForm", used)
	if first != "OpenaiForm" || second != "OpenaiForm_2" {
		t.Fatalf("names = %q, %q; want distinct stable identifiers", first, second)
	}
}
