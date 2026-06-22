// This file defines object reference records for stable cross-module links.
package ref

import "time"

type ObjectType string

const (
	ObjectTypeNote           ObjectType = "note"
	ObjectTypeFileCollection ObjectType = "file_collection"
	ObjectTypeFile           ObjectType = "file"
	ObjectTypeEventAggregate ObjectType = "event_aggregate"
	ObjectTypeEvent          ObjectType = "event"
	ObjectTypeAccount        ObjectType = "account"
	ObjectTypeTransaction    ObjectType = "transaction"
	ObjectTypeLLMSession     ObjectType = "llm_session"
	ObjectTypeLLMRequest     ObjectType = "llm_request"
)

type Module string

const (
	ModuleFiles      Module = "files"
	ModuleNotes      Module = "notes"
	ModuleAccounting Module = "accounting"
	ModuleCalendar   Module = "calendar"
	ModuleLLM        Module = "llm"
)

type ObjectRef struct {
	ID         int64
	OwnerID    int64
	RefCode    string
	ObjectType ObjectType
	ObjectID   int64
	Title      string
	Tags       []string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Registration struct {
	OwnerID    int64
	RefCode    string
	ObjectType ObjectType
	ObjectID   int64
	Title      string
	Tags       []string
	Status     string
}

type ProjectionUpdate struct {
	OwnerID    int64
	ObjectType ObjectType
	ObjectID   int64
	Title      string
	Tags       []string
	Status     string
}

type Metadata struct {
	RefCode    string     `json:"ref_code"`
	Module     Module     `json:"module"`
	ObjectType ObjectType `json:"object_type"`
	Title      string     `json:"title"`
	Tags       []string   `json:"tags"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

const (
	DefaultRecentMetadataLimit = 10
	MaxRecentMetadataLimit     = 50
)

type MetadataSearchQuery struct {
	Modules     []Module
	ObjectTypes []ObjectType
	Statuses    []string
	Tags        []string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	UpdatedFrom *time.Time
	UpdatedTo   *time.Time
	Sort        MetadataSearchSort
	Limit       int
}

type MetadataSearchSort struct {
	Field     MetadataSearchSortField
	Direction MetadataSearchSortDirection
}

type MetadataSearchSortField string

const (
	MetadataSearchSortCreatedAt MetadataSearchSortField = "created_at"
	MetadataSearchSortUpdatedAt MetadataSearchSortField = "updated_at"
	MetadataSearchSortRefCode   MetadataSearchSortField = "ref_code"
)

type MetadataSearchSortDirection string

const (
	MetadataSearchSortAscending  MetadataSearchSortDirection = "asc"
	MetadataSearchSortDescending MetadataSearchSortDirection = "desc"
)

const (
	DefaultMetadataSearchLimit = 50
	MaxMetadataSearchLimit     = 100
)
