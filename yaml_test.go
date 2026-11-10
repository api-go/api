package api_test

import (
	"strings"
	"testing"

	"github.com/api-go/api"
)

func TestParseActionYAML(t *testing.T) {
	definition, err := api.ParseActionYAML("name: sample\nmethod: POST\nstream: false\n")
	if err != nil {
		t.Fatal(err)
	}
	if definition["name"] != "sample" || definition["method"] != "POST" || definition["stream"] != false {
		t.Fatalf("unexpected definition: %#v", definition)
	}
	if _, err = api.ParseActionYAML("- invalid\n"); err == nil {
		t.Fatal("sequence Action must be rejected")
	}
}

func TestPatchActionTokensPreservesDocument(t *testing.T) {
	source := "# action comment\nname: sample\n# endpoint comment\nurl: https://example.com\ntokens: [old]\n"
	updated, err := api.PatchActionTokens(source, []string{"one", "two"})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# action comment", "# endpoint comment", "name: sample", "tokens: [one, two]"} {
		if !strings.Contains(updated, expected) {
			t.Fatalf("patched YAML does not contain %q:\n%s", expected, updated)
		}
	}
	removed, err := api.PatchActionTokens(updated, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(removed, "tokens:") {
		t.Fatalf("tokens field was not removed:\n%s", removed)
	}
}

func TestConvertActionJSONToYAML(t *testing.T) {
	converted, err := api.ConvertActionJSONToYAML(`{"name":"sample","headers":{"Accept":"application/json"},"filters":["clean"]}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# API Action configuration", "name: sample", "headers:\n  Accept: application/json", "filters:\n  - clean"} {
		if !strings.Contains(converted, expected) {
			t.Fatalf("converted YAML does not contain %q:\n%s", expected, converted)
		}
	}
}
