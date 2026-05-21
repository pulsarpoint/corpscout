package db

import (
	"reflect"
	"regexp"
	"testing"
)

func TestRegistryRawInputUpsertsDeriveSourceNativeIDFromRegistryIdentifier(t *testing.T) {
	t.Run("CVR", func(t *testing.T) {
		assertNoField(t, reflect.TypeOf(UpsertCVRRawInputParams{}), "SourceNativeID")
		assertValuesStart(t, upsertCVRRawInput, "$1", "$2", "$2")
	})

	t.Run("Ariregister", func(t *testing.T) {
		assertNoField(t, reflect.TypeOf(UpsertAriregisterRawInputParams{}), "SourceNativeID")
		assertValuesStart(t, upsertAriregisterRawInput, "$1", "$2", "$2")
	})
}

func assertNoField(t *testing.T, typ reflect.Type, name string) {
	t.Helper()

	if _, ok := typ.FieldByName(name); ok {
		t.Fatalf("%s should not expose caller-controlled %s", typ.Name(), name)
	}
}

func assertValuesStart(t *testing.T, query string, args ...string) {
	t.Helper()

	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	pattern := `VALUES \(\s*` + regexp.QuoteMeta(args[0]) + `,\s*` + regexp.QuoteMeta(args[1]) + `,\s*` + regexp.QuoteMeta(args[2]) + `,`
	if !regexp.MustCompile(pattern).MatchString(normalized) {
		t.Fatalf("query values should start with %v; got:\n%s", args, normalized)
	}
}
