-- Restore environment-scoped route storage for the active network-service contract.
-- The live pre-production pg18-next cluster currently has a routes table without
-- public.routes.environment_id, but the active API/resource contract requires one route
-- set per (application_id, environment_id).

ALTER TABLE public.routes
    ADD COLUMN IF NOT EXISTS environment_id text;

UPDATE public.routes
SET environment_id = 'base'
WHERE environment_id IS NULL OR btrim(environment_id) = '';

ALTER TABLE public.routes
    ALTER COLUMN environment_id SET DEFAULT 'base';

ALTER TABLE public.routes
    ALTER COLUMN environment_id SET NOT NULL;
