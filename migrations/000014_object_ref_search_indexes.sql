-- This migration adds indexes for owner-only object reference search filters.
CREATE INDEX idx_object_refs_owner_created
ON object_refs (owner_id, created_at DESC, ref_code DESC);

CREATE INDEX idx_object_refs_owner_updated
ON object_refs (owner_id, updated_at DESC, ref_code DESC);

CREATE INDEX idx_object_refs_owner_object_type_updated
ON object_refs (owner_id, object_type, updated_at DESC, ref_code DESC);
