// This file validates owner-only metadata search filters.
package ref

import "strings"

func normalizeMetadataSearchQuery(query MetadataSearchQuery) (MetadataSearchQuery, bool, error) {
	if query.Limit == 0 {
		query.Limit = DefaultMetadataSearchLimit
	}
	if query.Limit < 1 || query.Limit > MaxMetadataSearchLimit {
		return MetadataSearchQuery{}, false, ErrInvalidMetadataSearchQuery
	}

	modules, err := normalizeMetadataSearchModules(query.Modules)
	if err != nil {
		return MetadataSearchQuery{}, false, err
	}
	objectTypes, err := normalizeMetadataSearchObjectTypes(query.ObjectTypes)
	if err != nil {
		return MetadataSearchQuery{}, false, err
	}
	query.ObjectTypes, err = objectTypesForMetadataSearch(modules, objectTypes)
	if err != nil {
		return MetadataSearchQuery{}, false, err
	}
	if len(modules) > 0 && len(query.ObjectTypes) == 0 {
		return query, true, nil
	}
	query.Modules = nil

	query.Statuses, err = normalizeMetadataSearchStrings(query.Statuses)
	if err != nil {
		return MetadataSearchQuery{}, false, err
	}
	query.Tags, err = normalizeMetadataSearchStrings(query.Tags)
	if err != nil {
		return MetadataSearchQuery{}, false, err
	}
	if query.CreatedFrom != nil && query.CreatedTo != nil && query.CreatedFrom.After(*query.CreatedTo) {
		return MetadataSearchQuery{}, false, ErrInvalidMetadataSearchQuery
	}
	if query.UpdatedFrom != nil && query.UpdatedTo != nil && query.UpdatedFrom.After(*query.UpdatedTo) {
		return MetadataSearchQuery{}, false, ErrInvalidMetadataSearchQuery
	}
	query.Sort, err = normalizeMetadataSearchSort(query.Sort)
	if err != nil {
		return MetadataSearchQuery{}, false, err
	}
	return query, false, nil
}

func normalizeMetadataSearchModules(values []Module) ([]Module, error) {
	modules := make([]Module, 0, len(values))
	seen := make(map[Module]struct{})
	for _, value := range values {
		module := Module(strings.TrimSpace(string(value)))
		if module == "" {
			return nil, ErrInvalidMetadataSearchQuery
		}
		if !validMetadataSearchModule(module) {
			return nil, ErrInvalidMetadataSearchQuery
		}
		if _, exists := seen[module]; exists {
			continue
		}
		seen[module] = struct{}{}
		modules = append(modules, module)
	}
	return modules, nil
}

func normalizeMetadataSearchObjectTypes(values []ObjectType) ([]ObjectType, error) {
	objectTypes := make([]ObjectType, 0, len(values))
	seen := make(map[ObjectType]struct{})
	for _, value := range values {
		objectType := ObjectType(strings.TrimSpace(string(value)))
		if objectType == "" {
			return nil, ErrInvalidMetadataSearchQuery
		}
		if _, ok := objectDefinitions[objectType]; !ok {
			return nil, ErrInvalidMetadataSearchQuery
		}
		if _, exists := seen[objectType]; exists {
			continue
		}
		seen[objectType] = struct{}{}
		objectTypes = append(objectTypes, objectType)
	}
	return objectTypes, nil
}

func objectTypesForMetadataSearch(modules []Module, objectTypes []ObjectType) ([]ObjectType, error) {
	if len(modules) == 0 {
		return objectTypes, nil
	}
	allowedByModule := make(map[ObjectType]struct{})
	for _, module := range modules {
		for _, objectType := range objectTypeOrder {
			definition := objectDefinitions[objectType]
			if definition.module == module {
				allowedByModule[objectType] = struct{}{}
			}
		}
	}
	if len(objectTypes) == 0 {
		result := make([]ObjectType, 0, len(allowedByModule))
		for _, objectType := range objectTypeOrder {
			if _, ok := allowedByModule[objectType]; ok {
				result = append(result, objectType)
			}
		}
		return result, nil
	}
	requested := make(map[ObjectType]struct{}, len(objectTypes))
	for _, objectType := range objectTypes {
		requested[objectType] = struct{}{}
	}
	result := make([]ObjectType, 0, len(objectTypes))
	for _, objectType := range objectTypeOrder {
		if _, moduleAllowed := allowedByModule[objectType]; !moduleAllowed {
			continue
		}
		if _, requestedAllowed := requested[objectType]; requestedAllowed {
			result = append(result, objectType)
		}
	}
	return result, nil
}

func normalizeMetadataSearchStrings(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, ErrInvalidMetadataSearchQuery
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func normalizeMetadataSearchSort(sort MetadataSearchSort) (MetadataSearchSort, error) {
	if sort.Field == "" {
		sort.Field = MetadataSearchSortUpdatedAt
	}
	if sort.Direction == "" {
		sort.Direction = MetadataSearchSortDescending
	}
	switch sort.Field {
	case MetadataSearchSortCreatedAt, MetadataSearchSortUpdatedAt, MetadataSearchSortRefCode:
	default:
		return MetadataSearchSort{}, ErrInvalidMetadataSearchQuery
	}
	switch sort.Direction {
	case MetadataSearchSortAscending, MetadataSearchSortDescending:
	default:
		return MetadataSearchSort{}, ErrInvalidMetadataSearchQuery
	}
	return sort, nil
}

func validMetadataSearchModule(module Module) bool {
	switch module {
	case ModuleAccounting, ModuleCalendar, ModuleFiles, ModuleLLM, ModuleNotes:
		return true
	default:
		return false
	}
}
