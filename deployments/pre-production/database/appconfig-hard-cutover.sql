-- Hard cutover for AppConfig schema.
-- Apply this once to an existing legacy database before deploying the new AppConfig-only binaries.
-- This migration collapses legacy per-name configs into one record per (application_id, env)
-- and preserves the effective current payload as a single latest revision.

BEGIN;

CREATE OR REPLACE FUNCTION public.devflow_legacy_appconfig_filename(legacy_name text, legacy_format text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
    base_name text := regexp_replace(btrim(coalesce(legacy_name, '')), '\s+', '-', 'g');
    ext text := lower(btrim(coalesce(legacy_format, '')));
BEGIN
    IF base_name = '' THEN
        base_name := 'config';
    END IF;
    IF position('.' in base_name) > 0 THEN
        RETURN base_name;
    END IF;
    CASE ext
        WHEN 'yaml' THEN RETURN base_name || '.yaml';
        WHEN 'yml' THEN RETURN base_name || '.yaml';
        WHEN 'json' THEN RETURN base_name || '.json';
        WHEN 'toml' THEN RETURN base_name || '.toml';
        WHEN 'ini' THEN RETURN base_name || '.ini';
        WHEN 'properties' THEN RETURN base_name || '.properties';
        WHEN 'env' THEN RETURN base_name || '.env';
        ELSE RETURN base_name || '.conf';
    END CASE;
END
$$;

DO $$
DECLARE
    has_legacy_name boolean;
BEGIN
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'configurations'
          AND column_name = 'name'
    ) INTO has_legacy_name;

    IF NOT has_legacy_name THEN
        RAISE NOTICE 'legacy AppConfig columns already removed; skipping legacy data rewrite';
        RETURN;
    END IF;

    CREATE TEMP TABLE tmp_legacy_appconfig_rows ON COMMIT DROP AS
    SELECT *
    FROM public.configurations
    WHERE deleted_at IS NULL;

    CREATE TEMP TABLE tmp_appconfig_groups ON COMMIT DROP AS
    WITH ranked AS (
        SELECT
            c.id,
            c.application_id,
            c.env,
            c.source_path,
            c.mount_path,
            c.created_at,
            c.updated_at,
            row_number() OVER (
                PARTITION BY c.application_id, c.env
                ORDER BY c.updated_at DESC, c.created_at DESC, c.id DESC
            ) AS rn
        FROM tmp_legacy_appconfig_rows c
    )
    SELECT
        application_id,
        env,
        id AS canonical_id,
        coalesce(source_path, '') AS source_path,
        CASE
            WHEN btrim(coalesce(mount_path, '')) IN ('', '/etc/devflow/config', '/etc/devflow/config/') THEN '/etc/config'
            ELSE btrim(mount_path)
        END AS mount_path
    FROM ranked
    WHERE rn = 1;

    CREATE TEMP TABLE tmp_appconfig_file_candidates ON COMMIT DROP AS
    WITH base AS (
        SELECT
            c.application_id,
            c.env,
            c.id AS legacy_id,
            c.created_at,
            c.updated_at,
            CASE
                WHEN jsonb_typeof(c.files) = 'array' AND jsonb_array_length(c.files) > 0 THEN c.files
                WHEN btrim(coalesce(c.data, '')) = '' THEN '[]'::jsonb
                ELSE jsonb_build_array(
                    jsonb_build_object(
                        'name', public.devflow_legacy_appconfig_filename(c.name, c.format),
                        'content', c.data
                    )
                )
            END AS effective_files
        FROM tmp_legacy_appconfig_rows c
    ), expanded AS (
        SELECT
            b.application_id,
            b.env,
            b.legacy_id,
            b.created_at,
            b.updated_at,
            ordinality::integer AS ord,
            nullif(btrim(file_item->>'name'), '') AS file_name,
            coalesce(file_item->>'content', '') AS file_content
        FROM base b
        CROSS JOIN LATERAL jsonb_array_elements(b.effective_files) WITH ORDINALITY AS files(file_item, ordinality)
    )
    SELECT *
    FROM expanded;

    CREATE TEMP TABLE tmp_appconfig_merged_files ON COMMIT DROP AS
    WITH ranked AS (
        SELECT
            application_id,
            env,
            file_name,
            file_content,
            row_number() OVER (
                PARTITION BY application_id, env, file_name
                ORDER BY updated_at DESC, created_at DESC, ord DESC, legacy_id DESC
            ) AS rn
        FROM tmp_appconfig_file_candidates
        WHERE file_name IS NOT NULL
    )
    SELECT
        application_id,
        env,
        coalesce(
            jsonb_agg(
                jsonb_build_object('name', file_name, 'content', file_content)
                ORDER BY file_name
            ),
            '[]'::jsonb
        ) AS files_json
    FROM ranked
    WHERE rn = 1
    GROUP BY application_id, env;

    UPDATE public.configurations c
    SET deleted_at = now(), updated_at = now()
    FROM tmp_appconfig_groups g
    WHERE c.application_id = g.application_id
      AND c.env = g.env
      AND c.id <> g.canonical_id
      AND c.deleted_at IS NULL;

    DELETE FROM public.configuration_revisions r
    USING tmp_legacy_appconfig_rows legacy
    WHERE r.configuration_id = legacy.id;

    UPDATE public.configurations c
    SET latest_revision_no = 0,
        latest_revision_id = NULL,
        source_path = g.source_path,
        mount_path = g.mount_path,
        updated_at = now()
    FROM tmp_appconfig_groups g
    WHERE c.id = g.canonical_id;

    INSERT INTO public.configuration_revisions (
        id,
        configuration_id,
        revision_no,
        files,
        content_hash,
        message,
        created_by,
        created_at,
        source_commit,
        source_digest
    )
    SELECT
        gen_random_uuid(),
        g.canonical_id,
        1,
        coalesce(m.files_json, '[]'::jsonb),
        encode(digest(convert_to(coalesce(m.files_json, '[]'::jsonb)::text, 'UTF8'), 'sha256'), 'hex'),
        'hard cutover from legacy AppConfig schema',
        'migration',
        now(),
        '',
        encode(digest(convert_to(coalesce(m.files_json, '[]'::jsonb)::text, 'UTF8'), 'sha256'), 'hex')
    FROM tmp_appconfig_groups g
    LEFT JOIN tmp_appconfig_merged_files m
      ON m.application_id = g.application_id
     AND m.env = g.env;

    UPDATE public.configurations c
    SET latest_revision_no = 1,
        latest_revision_id = r.id,
        updated_at = now()
    FROM public.configuration_revisions r
    WHERE c.id = r.configuration_id
      AND r.revision_no = 1
      AND EXISTS (
          SELECT 1
          FROM tmp_appconfig_groups g
          WHERE g.canonical_id = c.id
      );
END
$$;

DROP INDEX IF EXISTS public.uq_configurations_app_env_name_active;
DROP INDEX IF EXISTS public.uq_configurations_app_env_active;
CREATE UNIQUE INDEX uq_configurations_app_env_active
    ON public.configurations USING btree (application_id, env)
    WHERE (deleted_at IS NULL);

UPDATE public.configurations
SET mount_path = '/etc/config'
WHERE btrim(coalesce(mount_path, '')) IN ('', '/etc/devflow/config', '/etc/devflow/config/');

ALTER TABLE public.configurations
    ALTER COLUMN latest_revision_no SET DEFAULT 0,
    ALTER COLUMN mount_path SET DEFAULT '/etc/config'::text;

ALTER TABLE public.configuration_revisions
    DROP COLUMN IF EXISTS rendered_configmap;

ALTER TABLE public.configurations
    DROP COLUMN IF EXISTS name,
    DROP COLUMN IF EXISTS files,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS format,
    DROP COLUMN IF EXISTS data,
    DROP COLUMN IF EXISTS labels;

DROP TABLE IF EXISTS public.environment_app_config_bindings;
DROP FUNCTION IF EXISTS public.devflow_legacy_appconfig_filename(text, text);

COMMIT;
