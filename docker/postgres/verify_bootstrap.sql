-- This script verifies Saturn schema ownership and the runtime role's least-privilege grant boundary.
DO $verify$
DECLARE
    target_table TEXT;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'saturn_owner'
          AND NOT rolcanlogin
          AND NOT rolsuper
          AND NOT rolcreatedb
          AND NOT rolcreaterole
          AND NOT rolreplication
          AND NOT rolbypassrls
    ) THEN
        RAISE EXCEPTION 'saturn_owner role attributes are incorrect';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'saturn'
          AND rolcanlogin
          AND NOT rolsuper
          AND NOT rolcreatedb
          AND NOT rolcreaterole
          AND NOT rolreplication
          AND NOT rolbypassrls
    ) THEN
        RAISE EXCEPTION 'saturn runtime role attributes are incorrect';
    END IF;

    IF pg_has_role('saturn', 'saturn_owner', 'MEMBER') THEN
        RAISE EXCEPTION 'saturn must not be a member of saturn_owner';
    END IF;

    IF (SELECT datdba::regrole::text FROM pg_database WHERE datname = current_database()) <> 'saturn_owner' THEN
        RAISE EXCEPTION 'Saturn database is not owned by saturn_owner';
    END IF;

    IF (SELECT nspowner::regrole::text FROM pg_namespace WHERE nspname = 'public') <> 'saturn_owner' THEN
        RAISE EXCEPTION 'public schema is not owned by saturn_owner';
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_tables
        WHERE schemaname = 'public' AND tableowner <> 'saturn_owner'
    ) THEN
        RAISE EXCEPTION 'a public table is not owned by saturn_owner';
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_sequences
        WHERE schemaname = 'public' AND sequenceowner <> 'saturn_owner'
    ) THEN
        RAISE EXCEPTION 'a public sequence is not owned by saturn_owner';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_proc
        JOIN pg_namespace ON pg_namespace.oid = pg_proc.pronamespace
        WHERE nspname = 'public' AND proowner::regrole::text <> 'saturn_owner'
    ) THEN
        RAISE EXCEPTION 'a public function is not owned by saturn_owner';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_trigger
        JOIN pg_class ON pg_class.oid = pg_trigger.tgrelid
        JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
        WHERE nspname = 'public'
          AND NOT tgisinternal
          AND tgenabled <> 'O'
    ) THEN
        RAISE EXCEPTION 'a Saturn trigger is not enabled in origin/local mode';
    END IF;

    IF NOT has_database_privilege('saturn', current_database(), 'CONNECT')
       OR has_database_privilege('saturn', current_database(), 'CREATE')
       OR has_database_privilege('saturn', current_database(), 'TEMP') THEN
        RAISE EXCEPTION 'Saturn database privileges are incorrect';
    END IF;

    IF NOT has_schema_privilege('saturn', 'public', 'USAGE')
       OR has_schema_privilege('saturn', 'public', 'CREATE') THEN
        RAISE EXCEPTION 'Saturn schema privileges are incorrect';
    END IF;

    IF NOT has_table_privilege('saturn', 'audit_logs', 'SELECT')
       OR NOT has_column_privilege('saturn', 'audit_logs', 'actor_ref_code', 'INSERT')
       OR has_table_privilege('saturn', 'audit_logs', 'UPDATE')
       OR has_table_privilege('saturn', 'audit_logs', 'DELETE')
       OR has_table_privilege('saturn', 'audit_logs', 'TRUNCATE') THEN
        RAISE EXCEPTION 'Saturn audit privileges are incorrect';
    END IF;

    IF NOT has_column_privilege('saturn', 'transactions', 'status', 'UPDATE')
       OR has_column_privilege('saturn', 'transactions', 'amount_cents', 'UPDATE') THEN
        RAISE EXCEPTION 'Saturn transaction update privileges are incorrect';
    END IF;

    IF has_function_privilege('saturn', 'reject_audit_log_mutation()', 'EXECUTE') THEN
        RAISE EXCEPTION 'Saturn must not execute trigger functions directly';
    END IF;

    FOREACH target_table IN ARRAY ARRAY[
        'note_collections',
        'note_collection_items',
        'note_links',
        'note_templates',
        'note_sources',
        'rss_sources',
        'search_documents',
        'search_index_queue_jobs',
        'import_jobs',
        'export_jobs',
        'export_manifests',
        'restore_previews',
        'storage_diagnostics',
        'system_settings',
        'instance_preferences'
    ] LOOP
        IF has_any_column_privilege('saturn', target_table, 'SELECT,INSERT,UPDATE,REFERENCES')
           OR has_table_privilege('saturn', target_table, 'DELETE')
           OR has_table_privilege('saturn', target_table, 'TRUNCATE')
           OR has_table_privilege('saturn', target_table, 'TRIGGER') THEN
            RAISE EXCEPTION 'Saturn has an unexpected privilege on unused table %', target_table;
        END IF;
    END LOOP;

    IF to_regclass('public.note_versions') IS NULL
       OR NOT EXISTS (
           SELECT 1
           FROM information_schema.columns
           WHERE table_schema = 'public'
             AND table_name = 'events'
             AND column_name = 'ends_at'
       )
       OR EXISTS (
           SELECT 1
           FROM information_schema.columns
           WHERE table_schema = 'public'
             AND table_name = 'events'
             AND column_name = 'duration_minutes'
       ) THEN
        RAISE EXCEPTION 'the complete migration set was not applied';
    END IF;
END
$verify$;
