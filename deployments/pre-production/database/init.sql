--
-- PostgreSQL database dump
--


-- Dumped from database version 18.3 (Debian 18.3-1.pgdg13+1)
-- Dumped by pg_dump version 18.3 (Debian 18.3-1.pgdg13+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: application_environment_bindings; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.application_environment_bindings (
    application_id text NOT NULL,
    environment_id text NOT NULL,
    binding_id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);


ALTER TABLE public.application_environment_bindings OWNER TO app;

--
-- Name: application_runtime_spec_revisions; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.application_runtime_spec_revisions (
    id uuid NOT NULL,
    runtime_spec_id uuid NOT NULL,
    revision integer NOT NULL,
    replicas integer NOT NULL,
    health_thresholds_jsonb jsonb DEFAULT '{}'::jsonb CONSTRAINT application_runtime_spec_revis_health_thresholds_jsonb_not_null NOT NULL,
    resources_jsonb jsonb DEFAULT '{}'::jsonb NOT NULL,
    autoscaling_jsonb jsonb DEFAULT '{}'::jsonb NOT NULL,
    scheduling_jsonb jsonb DEFAULT '{}'::jsonb NOT NULL,
    pod_envs_jsonb jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_by text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone NOT NULL
);


ALTER TABLE public.application_runtime_spec_revisions OWNER TO app;

--
-- Name: application_runtime_specs; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.application_runtime_specs (
    id uuid NOT NULL,
    application_id uuid NOT NULL,
    environment text NOT NULL,
    current_revision_id uuid,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL
);


ALTER TABLE public.application_runtime_specs OWNER TO app;

--
-- Name: applications; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.applications (
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    name text NOT NULL,
    repo_address text NOT NULL,
    labels jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone,
    description text DEFAULT ''::text NOT NULL
);


ALTER TABLE public.applications OWNER TO app;

--
-- Name: clusters; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.clusters (
    id uuid NOT NULL,
    name text NOT NULL,
    server text NOT NULL,
    kubeconfig text NOT NULL,
    argocd_cluster_name text DEFAULT ''::text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    labels jsonb DEFAULT '[]'::jsonb NOT NULL,
    onboarding_ready boolean DEFAULT false NOT NULL,
    onboarding_error text DEFAULT ''::text NOT NULL,
    onboarding_checked_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone
);


ALTER TABLE public.clusters OWNER TO app;

--
-- Name: configuration_revisions; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.configuration_revisions (
    id uuid NOT NULL,
    configuration_id uuid NOT NULL,
    revision_no integer NOT NULL,
    files jsonb DEFAULT '[]'::jsonb NOT NULL,
    content_hash text NOT NULL,
    message text DEFAULT ''::text NOT NULL,
    created_by text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    source_commit text DEFAULT ''::text NOT NULL,
    source_digest text DEFAULT ''::text NOT NULL,
    rendered_configmap jsonb DEFAULT '{"data": {}}'::jsonb NOT NULL
);


ALTER TABLE public.configuration_revisions OWNER TO app;

--
-- Name: configurations; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.configurations (
    id uuid NOT NULL,
    application_id uuid NOT NULL,
    name text NOT NULL,
    env text NOT NULL,
    latest_revision_no integer DEFAULT 1 NOT NULL,
    latest_revision_id uuid,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone,
    source_path text DEFAULT ''::text NOT NULL,
    files jsonb DEFAULT '[]'::jsonb NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    format text DEFAULT ''::text NOT NULL,
    data text DEFAULT ''::text NOT NULL,
    labels jsonb DEFAULT '[]'::jsonb NOT NULL,
    mount_path text DEFAULT '/etc/devflow/config'::text NOT NULL
);


ALTER TABLE public.configurations OWNER TO app;

--
-- Name: environment_app_config_bindings; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.environment_app_config_bindings (
    application_id text NOT NULL,
    environment_id text NOT NULL,
    binding_id uuid DEFAULT gen_random_uuid() NOT NULL,
    base_configuration_id text DEFAULT ''::text NOT NULL,
    config_service_id text DEFAULT ''::text NOT NULL,
    name_override text,
    description_override text,
    format_override text,
    data_override text,
    labels_override_set boolean DEFAULT false NOT NULL,
    labels_override_json jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.environment_app_config_bindings OWNER TO app;

--
-- Name: environment_workload_config_bindings; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.environment_workload_config_bindings (
    application_id text NOT NULL,
    environment_id text NOT NULL,
    binding_id uuid DEFAULT gen_random_uuid() NOT NULL,
    base_deployment_config_id text DEFAULT ''::text CONSTRAINT environment_workload_config__base_deployment_config_id_not_null NOT NULL,
    name_override text,
    description_override text,
    replicas_override integer,
    exposed_override boolean,
    strategy_override text,
    labels_override_set boolean DEFAULT false CONSTRAINT environment_workload_config_bindin_labels_override_set_not_null NOT NULL,
    labels_override_json jsonb DEFAULT '[]'::jsonb CONSTRAINT environment_workload_config_bindi_labels_override_json_not_null NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.environment_workload_config_bindings OWNER TO app;

--
-- Name: environments; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.environments (
    id uuid NOT NULL,
    name text NOT NULL,
    labels jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone,
    description text DEFAULT ''::text NOT NULL,
    cluster_id uuid NOT NULL
);


ALTER TABLE public.environments OWNER TO app;

--
-- Name: execution_intents; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.execution_intents (
    id uuid NOT NULL,
    kind text NOT NULL,
    status text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid NOT NULL,
    trace_id text DEFAULT ''::text NOT NULL,
    message text DEFAULT ''::text NOT NULL,
    last_error text DEFAULT ''::text NOT NULL,
    claimed_by text DEFAULT ''::text NOT NULL,
    claimed_at timestamp with time zone,
    lease_expires_at timestamp with time zone,
    attempt_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone
);


ALTER TABLE public.execution_intents OWNER TO app;


--
-- Name: manifest_verifications; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.manifest_verifications (
    id uuid NOT NULL,
    manifest_id uuid NOT NULL,
    intent_id uuid,
    pipeline_id text DEFAULT ''::text NOT NULL,
    status text NOT NULL,
    external_ref text DEFAULT ''::text NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    last_message text DEFAULT ''::text NOT NULL,
    steps jsonb DEFAULT '[]'::jsonb NOT NULL,
    details jsonb DEFAULT '{}'::jsonb NOT NULL,
    last_observed_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL
);


ALTER TABLE public.manifest_verifications OWNER TO app;

--
-- Name: manifests; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.manifests (
    id uuid CONSTRAINT manifests_id_not_null1 NOT NULL,
    application_id uuid CONSTRAINT manifests_application_id_not_null1 NOT NULL,
    git_revision text DEFAULT ''::text NOT NULL,
    repo_address text DEFAULT ''::text NOT NULL,
    commit_hash text DEFAULT ''::text NOT NULL,
    image_ref text NOT NULL,
    image_tag text DEFAULT ''::text NOT NULL,
    image_digest text DEFAULT ''::text NOT NULL,
    pipeline_id text DEFAULT ''::text NOT NULL,
    trace_id text DEFAULT ''::text NOT NULL,
    span_id text DEFAULT ''::text NOT NULL,
    steps jsonb DEFAULT '[]'::jsonb NOT NULL,
    services_snapshot jsonb DEFAULT '[]'::jsonb NOT NULL,
    workload_config_snapshot jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text CONSTRAINT manifests_status_not_null1 NOT NULL,
    created_at timestamp with time zone CONSTRAINT manifests_created_at_not_null1 NOT NULL,
    updated_at timestamp with time zone CONSTRAINT manifests_updated_at_not_null1 NOT NULL,
    deleted_at timestamp with time zone
);


ALTER TABLE public.manifests OWNER TO app;

--
-- Name: networks; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.networks (
    id uuid NOT NULL,
    application_id uuid NOT NULL,
    name text NOT NULL,
    ports jsonb DEFAULT '[]'::jsonb NOT NULL,
    hosts jsonb DEFAULT '[]'::jsonb NOT NULL,
    paths jsonb DEFAULT '[]'::jsonb NOT NULL,
    gateway_refs jsonb DEFAULT '[]'::jsonb NOT NULL,
    visibility text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone
);


ALTER TABLE public.networks OWNER TO app;

--
-- Name: projects; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.projects (
    id uuid NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    labels jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone
);


ALTER TABLE public.projects OWNER TO app;

--
-- Name: release_verifications; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.release_verifications (
    id uuid NOT NULL,
    release_id uuid NOT NULL,
    intent_id uuid,
    env text DEFAULT ''::text NOT NULL,
    status text NOT NULL,
    external_ref text DEFAULT ''::text NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    last_message text DEFAULT ''::text NOT NULL,
    steps jsonb DEFAULT '[]'::jsonb NOT NULL,
    details jsonb DEFAULT '{}'::jsonb NOT NULL,
    last_observed_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL
);


ALTER TABLE public.release_verifications OWNER TO app;

--
-- Name: releases; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.releases (
    id uuid NOT NULL,
    execution_intent_id uuid,
    application_id uuid NOT NULL,
    env text NOT NULL,
    strategy text DEFAULT 'rolling'::text NOT NULL,
    routes_snapshot jsonb DEFAULT '[]'::jsonb NOT NULL,
    app_config_snapshot jsonb DEFAULT '{}'::jsonb NOT NULL,
    artifact_repository text DEFAULT ''::text NOT NULL,
    artifact_tag text DEFAULT ''::text NOT NULL,
    artifact_digest text DEFAULT ''::text NOT NULL,
    artifact_ref text DEFAULT ''::text NOT NULL,
    type text NOT NULL,
    steps jsonb DEFAULT '[]'::jsonb NOT NULL,
    status text NOT NULL,
    argocd_application_name text DEFAULT ''::text NOT NULL,
    external_ref text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone,
    manifest_id uuid CONSTRAINT releases_manifest_id_not_null1 NOT NULL
);


ALTER TABLE public.releases OWNER TO app;

--
-- Name: routes; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.routes (
    id uuid NOT NULL,
    application_id uuid NOT NULL,
    environment_id text NOT NULL DEFAULT 'base'::text,
    name text NOT NULL,
    host text NOT NULL,
    path text NOT NULL,
    service_name text NOT NULL,
    service_port integer NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone
);


ALTER TABLE public.routes OWNER TO app;

--
-- Name: runtime_observed_pods; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.runtime_observed_pods (
    id uuid NOT NULL,
    runtime_spec_id uuid NOT NULL,
    application_id uuid NOT NULL,
    environment text NOT NULL,
    namespace text NOT NULL,
    pod_name text NOT NULL,
    phase text DEFAULT ''::text NOT NULL,
    ready boolean DEFAULT false NOT NULL,
    restarts integer DEFAULT 0 NOT NULL,
    node_name text DEFAULT ''::text NOT NULL,
    pod_ip text DEFAULT ''::text NOT NULL,
    host_ip text DEFAULT ''::text NOT NULL,
    owner_kind text DEFAULT ''::text NOT NULL,
    owner_name text DEFAULT ''::text NOT NULL,
    labels_jsonb jsonb DEFAULT '{}'::jsonb NOT NULL,
    containers_jsonb jsonb DEFAULT '[]'::jsonb NOT NULL,
    observed_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone
);


ALTER TABLE public.runtime_observed_pods OWNER TO app;

--
-- Name: services; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.services (
    id uuid NOT NULL,
    application_id uuid NOT NULL,
    name text NOT NULL,
    exposure text NOT NULL,
    ports jsonb DEFAULT '[]'::jsonb NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone,
    description text DEFAULT ''::text NOT NULL,
    labels jsonb DEFAULT '{}'::jsonb NOT NULL
);


ALTER TABLE public.services OWNER TO app;

--
-- Name: workload_configs; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.workload_configs (
    id uuid NOT NULL,
    application_id uuid NOT NULL,
    environment_id text,
    name text NOT NULL,
    replicas integer DEFAULT 1 NOT NULL,
    resources jsonb DEFAULT '{}'::jsonb NOT NULL,
    probes jsonb DEFAULT '{}'::jsonb NOT NULL,
    env jsonb DEFAULT '[]'::jsonb NOT NULL,
    workload_type text DEFAULT ''::text NOT NULL,
    strategy text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    deleted_at timestamp with time zone,
    description text DEFAULT ''::text NOT NULL,
    service_account_name text DEFAULT ''::text NOT NULL,
    labels jsonb DEFAULT '[]'::jsonb NOT NULL
);


ALTER TABLE public.workload_configs OWNER TO app;

--
-- Name: application_environment_bindings application_environment_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.application_environment_bindings
    ADD CONSTRAINT application_environment_bindings_pkey PRIMARY KEY (application_id, environment_id);


--
-- Name: application_runtime_spec_revisions application_runtime_spec_revisions_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.application_runtime_spec_revisions
    ADD CONSTRAINT application_runtime_spec_revisions_pkey PRIMARY KEY (id);


--
-- Name: application_runtime_spec_revisions application_runtime_spec_revisions_runtime_spec_id_revision_key; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.application_runtime_spec_revisions
    ADD CONSTRAINT application_runtime_spec_revisions_runtime_spec_id_revision_key UNIQUE (runtime_spec_id, revision);


--
-- Name: application_runtime_specs application_runtime_specs_application_id_environment_key; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.application_runtime_specs
    ADD CONSTRAINT application_runtime_specs_application_id_environment_key UNIQUE (application_id, environment);


--
-- Name: application_runtime_specs application_runtime_specs_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.application_runtime_specs
    ADD CONSTRAINT application_runtime_specs_pkey PRIMARY KEY (id);


--
-- Name: applications applications_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.applications
    ADD CONSTRAINT applications_pkey PRIMARY KEY (id);


--
-- Name: clusters clusters_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.clusters
    ADD CONSTRAINT clusters_pkey PRIMARY KEY (id);


--
-- Name: configuration_revisions configuration_revisions_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.configuration_revisions
    ADD CONSTRAINT configuration_revisions_pkey PRIMARY KEY (id);


--
-- Name: configurations configurations_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.configurations
    ADD CONSTRAINT configurations_pkey PRIMARY KEY (id);


--
-- Name: environment_app_config_bindings environment_app_config_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.environment_app_config_bindings
    ADD CONSTRAINT environment_app_config_bindings_pkey PRIMARY KEY (application_id, environment_id);


--
-- Name: environment_workload_config_bindings environment_workload_config_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.environment_workload_config_bindings
    ADD CONSTRAINT environment_workload_config_bindings_pkey PRIMARY KEY (application_id, environment_id);


--
-- Name: environments environments_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.environments
    ADD CONSTRAINT environments_pkey PRIMARY KEY (id);


--
-- Name: execution_intents execution_intents_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.execution_intents
    ADD CONSTRAINT execution_intents_pkey PRIMARY KEY (id);


--

--
-- Name: manifest_verifications manifest_verifications_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.manifest_verifications
    ADD CONSTRAINT manifest_verifications_pkey PRIMARY KEY (id);


--
-- Name: manifests manifests_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.manifests
    ADD CONSTRAINT manifests_pkey PRIMARY KEY (id);


--
-- Name: networks networks_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.networks
    ADD CONSTRAINT networks_pkey PRIMARY KEY (id);


--
-- Name: projects projects_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_pkey PRIMARY KEY (id);


--
-- Name: release_verifications release_verifications_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.release_verifications
    ADD CONSTRAINT release_verifications_pkey PRIMARY KEY (id);


--
-- Name: releases releases_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.releases
    ADD CONSTRAINT releases_pkey PRIMARY KEY (id);


--
-- Name: routes routes_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.routes
    ADD CONSTRAINT routes_pkey PRIMARY KEY (id);


--
-- Name: runtime_observed_pods runtime_observed_pods_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.runtime_observed_pods
    ADD CONSTRAINT runtime_observed_pods_pkey PRIMARY KEY (id);


--
-- Name: runtime_observed_pods runtime_observed_pods_runtime_spec_id_namespace_pod_name_key; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.runtime_observed_pods
    ADD CONSTRAINT runtime_observed_pods_runtime_spec_id_namespace_pod_name_key UNIQUE (runtime_spec_id, namespace, pod_name);


--
-- Name: services services_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.services
    ADD CONSTRAINT services_pkey PRIMARY KEY (id);


--
-- Name: workload_configs workload_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.workload_configs
    ADD CONSTRAINT workload_configs_pkey PRIMARY KEY (id);


--
-- Name: idx_applications_project_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_applications_project_id ON public.applications USING btree (project_id);


--
-- Name: idx_configuration_revisions_hash; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_configuration_revisions_hash ON public.configuration_revisions USING btree (content_hash);


--
-- Name: idx_configurations_application_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_configurations_application_id ON public.configurations USING btree (application_id);


--
-- Name: idx_environments_cluster_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_environments_cluster_id ON public.environments USING btree (cluster_id);


--
-- Name: idx_execution_intents_claimed_by; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_execution_intents_claimed_by ON public.execution_intents USING btree (claimed_by);


--
-- Name: idx_execution_intents_resource; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_execution_intents_resource ON public.execution_intents USING btree (resource_type, resource_id);


--
-- Name: idx_execution_intents_status_kind; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_execution_intents_status_kind ON public.execution_intents USING btree (status, kind);


--

--

--
-- Name: idx_manifest_verifications_intent_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_manifest_verifications_intent_id ON public.manifest_verifications USING btree (intent_id);


--
-- Name: idx_manifest_verifications_pipeline_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_manifest_verifications_pipeline_id ON public.manifest_verifications USING btree (pipeline_id);


--
-- Name: idx_manifests_application_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_manifests_application_id ON public.manifests USING btree (application_id);


--
-- Name: idx_networks_application_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_networks_application_id ON public.networks USING btree (application_id);


--
-- Name: idx_release_verifications_intent_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_release_verifications_intent_id ON public.release_verifications USING btree (intent_id);


--
-- Name: idx_releases_application_env; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_releases_application_env ON public.releases USING btree (application_id, env);


--
-- Name: idx_releases_application_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_releases_application_id ON public.releases USING btree (application_id);


--
-- Name: idx_releases_execution_intent_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_releases_execution_intent_id ON public.releases USING btree (execution_intent_id);


--
-- Name: idx_releases_manifest_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_releases_manifest_id ON public.releases USING btree (manifest_id);


--
-- Name: idx_routes_application_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_routes_application_id ON public.routes USING btree (application_id);


--
-- Name: idx_services_application_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_services_application_id ON public.services USING btree (application_id);


--
-- Name: idx_workload_configs_application_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_workload_configs_application_id ON public.workload_configs USING btree (application_id);


--
-- Name: uq_applications_project_name_active; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_applications_project_name_active ON public.applications USING btree (project_id, name) WHERE (deleted_at IS NULL);


--
-- Name: uq_clusters_name_active; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_clusters_name_active ON public.clusters USING btree (name) WHERE (deleted_at IS NULL);


--
-- Name: uq_configuration_revisions_no; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_configuration_revisions_no ON public.configuration_revisions USING btree (configuration_id, revision_no);


--
-- Name: uq_configurations_app_env_name_active; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_configurations_app_env_name_active ON public.configurations USING btree (application_id, env, name) WHERE (deleted_at IS NULL);


--
-- Name: uq_environments_name_active; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_environments_name_active ON public.environments USING btree (name) WHERE (deleted_at IS NULL);


--

--
-- Name: uq_manifest_verifications_manifest_id; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_manifest_verifications_manifest_id ON public.manifest_verifications USING btree (manifest_id);


--
-- Name: uq_networks_application_name_active; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_networks_application_name_active ON public.networks USING btree (application_id, name) WHERE (deleted_at IS NULL);


--
-- Name: uq_projects_name_active; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_projects_name_active ON public.projects USING btree (name) WHERE (deleted_at IS NULL);


--
-- Name: uq_release_verifications_release_id; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_release_verifications_release_id ON public.release_verifications USING btree (release_id);


--
-- Name: uq_routes_application_name_active; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_routes_application_name_active ON public.routes USING btree (application_id, name) WHERE (deleted_at IS NULL);


--
-- Name: uq_services_name_active; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_services_name_active ON public.services USING btree (application_id, name) WHERE (deleted_at IS NULL);


--
-- Name: uq_workload_configs_scope_name_active; Type: INDEX; Schema: public; Owner: app
--

CREATE UNIQUE INDEX uq_workload_configs_scope_name_active ON public.workload_configs USING btree (application_id, COALESCE(environment_id, ''::text), name) WHERE (deleted_at IS NULL);


--
-- Name: application_runtime_spec_revisions application_runtime_spec_revisions_runtime_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.application_runtime_spec_revisions
    ADD CONSTRAINT application_runtime_spec_revisions_runtime_spec_id_fkey FOREIGN KEY (runtime_spec_id) REFERENCES public.application_runtime_specs(id) ON DELETE CASCADE;


--
-- Name: environment_app_config_bindings environment_app_config_bindin_application_id_environment_i_fkey; Type: FK CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.environment_app_config_bindings
    ADD CONSTRAINT environment_app_config_bindin_application_id_environment_i_fkey FOREIGN KEY (application_id, environment_id) REFERENCES public.application_environment_bindings(application_id, environment_id) ON DELETE CASCADE;


--
-- Name: environment_workload_config_bindings environment_workload_config_b_application_id_environment_i_fkey; Type: FK CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.environment_workload_config_bindings
    ADD CONSTRAINT environment_workload_config_b_application_id_environment_i_fkey FOREIGN KEY (application_id, environment_id) REFERENCES public.application_environment_bindings(application_id, environment_id) ON DELETE CASCADE;


--
-- Name: runtime_observed_pods runtime_observed_pods_runtime_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.runtime_observed_pods
    ADD CONSTRAINT runtime_observed_pods_runtime_spec_id_fkey FOREIGN KEY (runtime_spec_id) REFERENCES public.application_runtime_specs(id) ON DELETE CASCADE;


--
-- Name: runtime_operations; Type: TABLE; Schema: public; Owner: app
--

CREATE TABLE public.runtime_operations (
    id uuid NOT NULL,
    runtime_spec_id uuid NOT NULL,
    operation_type text NOT NULL,
    target_name text NOT NULL,
    operator text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone NOT NULL
);


ALTER TABLE public.runtime_operations OWNER TO app;

--
-- Name: runtime_operations runtime_operations_pkey; Type: CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.runtime_operations
    ADD CONSTRAINT runtime_operations_pkey PRIMARY KEY (id);

--
-- Name: idx_runtime_operations_runtime_spec_id; Type: INDEX; Schema: public; Owner: app
--

CREATE INDEX idx_runtime_operations_runtime_spec_id ON public.runtime_operations USING btree (runtime_spec_id);

--
-- Name: runtime_operations runtime_operations_runtime_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: app
--

ALTER TABLE ONLY public.runtime_operations
    ADD CONSTRAINT runtime_operations_runtime_spec_id_fkey FOREIGN KEY (runtime_spec_id) REFERENCES public.application_runtime_specs(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--
