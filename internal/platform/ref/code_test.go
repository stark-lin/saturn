// This file tests object reference code formatting and navigation URLs.
package ref

import (
	"errors"
	"testing"
)

func TestFormatCodeUsesFixedTypePrefixesAndGlobalHexSequence(t *testing.T) {
	tests := []struct {
		objectType ObjectType
		want       string
	}{
		{objectType: ObjectTypeNote, want: "NTE-0000002A"},
		{objectType: ObjectTypeFileCollection, want: "FIL-0000002A"},
		{objectType: ObjectTypeFile, want: "FIL-0000002A"},
		{objectType: ObjectTypeEventAggregate, want: "CAL-0000002A"},
		{objectType: ObjectTypeEvent, want: "CAL-0000002A"},
		{objectType: ObjectTypeAccount, want: "ACC-0000002A"},
		{objectType: ObjectTypeTransaction, want: "ACC-0000002A"},
		{objectType: ObjectTypeLLMSession, want: "LLM-0000002A"},
		{objectType: ObjectTypeLLMRequest, want: "LLM-0000002A"},
	}

	for _, test := range tests {
		code, err := FormatCode(test.objectType, 42)
		if err != nil {
			t.Fatalf("format %s code: %v", test.objectType, err)
		}
		if code != test.want {
			t.Fatalf("format %s code = %q, want %q", test.objectType, code, test.want)
		}
	}
}

func TestFormatCodeRejectsUnsupportedTypesAndOutOfRangeSequences(t *testing.T) {
	if _, err := FormatCode(ObjectType("budget"), 1); !errors.Is(err, ErrUnsupportedObjectType) {
		t.Fatalf("unsupported type error = %v, want unsupported object type", err)
	}
	if _, err := FormatCode(ObjectTypeNote, 0); !errors.Is(err, ErrInvalidSequence) {
		t.Fatalf("zero sequence error = %v, want invalid sequence", err)
	}
}

func TestModuleForObjectTypeMapsSharedModulePrefixes(t *testing.T) {
	module, err := ModuleForObjectType(ObjectTypeTransaction)
	if err != nil {
		t.Fatalf("transaction module: %v", err)
	}
	if module != ModuleAccounting || !CodeMatchesObjectType("ACC-00000001", ObjectTypeAccount) {
		t.Fatalf("accounting module = %q or prefix mismatch", module)
	}
	module, err = ModuleForObjectType(ObjectTypeFileCollection)
	if err != nil {
		t.Fatalf("file collection module: %v", err)
	}
	if module != ModuleFiles || !CodeMatchesObjectType("FIL-00000001", ObjectTypeFileCollection) {
		t.Fatalf("files module = %q or file collection prefix mismatch", module)
	}
}
