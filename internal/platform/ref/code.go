// This file validates and formats Saturn object reference codes.
package ref

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var codePattern = regexp.MustCompile(`^[A-Z]{3}-[0-9A-F]{8}$`)

var (
	ErrUnsupportedObjectType = errors.New("unsupported object type")
	ErrInvalidSequence       = errors.New("invalid ref code sequence")
)

type objectDefinition struct {
	module Module
	prefix string
}

var objectDefinitions = map[ObjectType]objectDefinition{
	ObjectTypeUser:           {module: ModulePlatform, prefix: "USR"},
	ObjectTypeAPIKey:         {module: ModulePlatform, prefix: "KEY"},
	ObjectTypeNote:           {module: ModuleNotes, prefix: "NTE"},
	ObjectTypeNoteVersion:    {module: ModuleNotes, prefix: "NTE"},
	ObjectTypeFileCollection: {module: ModuleFiles, prefix: "FIL"},
	ObjectTypeFile:           {module: ModuleFiles, prefix: "FIL"},
	ObjectTypeEventAggregate: {module: ModuleCalendar, prefix: "CAL"},
	ObjectTypeEvent:          {module: ModuleCalendar, prefix: "CAL"},
	ObjectTypeAccount:        {module: ModuleAccounting, prefix: "ACC"},
	ObjectTypeTransaction:    {module: ModuleAccounting, prefix: "ACC"},
	ObjectTypeLLMSession:     {module: ModuleLLM, prefix: "LLM"},
	ObjectTypeLLMRequest:     {module: ModuleLLM, prefix: "LLM"},
}

func isMetadataObjectType(objectType ObjectType) bool {
	for _, metadataObjectType := range objectTypeOrder {
		if objectType == metadataObjectType {
			return true
		}
	}
	return false
}

func isIdentityObjectType(objectType ObjectType) bool {
	return objectType == ObjectTypeUser || objectType == ObjectTypeAPIKey
}

var objectTypeOrder = []ObjectType{
	ObjectTypeNote,
	ObjectTypeNoteVersion,
	ObjectTypeFileCollection,
	ObjectTypeFile,
	ObjectTypeEventAggregate,
	ObjectTypeEvent,
	ObjectTypeAccount,
	ObjectTypeTransaction,
	ObjectTypeLLMSession,
	ObjectTypeLLMRequest,
}

func FormatCode(objectType ObjectType, sequence int64) (string, error) {
	definition, ok := objectDefinitions[objectType]
	if !ok {
		return "", ErrUnsupportedObjectType
	}
	if sequence < 1 || sequence > 0xFFFFFFFF {
		return "", ErrInvalidSequence
	}
	return fmt.Sprintf("%s-%08X", definition.prefix, sequence), nil
}

func ModuleForObjectType(objectType ObjectType) (Module, error) {
	definition, ok := objectDefinitions[objectType]
	if !ok {
		return "", ErrUnsupportedObjectType
	}
	return definition.module, nil
}

func ValidCode(code string) bool {
	return codePattern.MatchString(code)
}

func NormalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func CodeMatchesObjectType(code string, objectType ObjectType) bool {
	definition, ok := objectDefinitions[objectType]
	if !ok {
		return false
	}
	return ValidCode(code) && strings.HasPrefix(code, definition.prefix+"-")
}

func CodeMatchesModule(code string, module Module) bool {
	if !ValidCode(code) {
		return false
	}
	for _, definition := range objectDefinitions {
		if definition.module == module && strings.HasPrefix(code, definition.prefix+"-") {
			return true
		}
	}
	return false
}
