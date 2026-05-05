ALTER TABLE public.workload_configs
    ADD COLUMN IF NOT EXISTS metrics jsonb DEFAULT '{}'::jsonb NOT NULL;
