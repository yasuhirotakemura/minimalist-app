--
-- PostgreSQL database dump
--

\restrict 8ocQoXeiE3qiPczVKyjvbJElEk5zdMFTzH0PMTcnZapZBF7gzsD75kXqjdxUON9

-- Dumped from database version 17.10
-- Dumped by pg_dump version 17.10

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
-- Name: audit; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA audit;


--
-- Name: identity; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA identity;


--
-- Name: ownership; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA ownership;


--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA public;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: audit_logs; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.audit_logs (
    id bigint NOT NULL,
    public_id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id bigint NOT NULL,
    action_code text NOT NULL,
    target_type_code text NOT NULL,
    target_public_id uuid,
    changes jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_audit_logs__action_code_shape CHECK ((action_code ~ '^[a-z][a-z0-9_]{0,62}$'::text)),
    CONSTRAINT ck_audit_logs__changes_is_object CHECK ((jsonb_typeof(changes) = 'object'::text)),
    CONSTRAINT ck_audit_logs__target_type_code_shape CHECK ((target_type_code ~ '^[a-z][a-z0-9_]{0,31}$'::text))
);


--
-- Name: audit_logs_id_seq; Type: SEQUENCE; Schema: audit; Owner: -
--

ALTER TABLE audit.audit_logs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME audit.audit_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: auth_sessions; Type: TABLE; Schema: identity; Owner: -
--

CREATE TABLE identity.auth_sessions (
    id bigint NOT NULL,
    public_id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id bigint NOT NULL,
    token_hash bytea NOT NULL,
    issued_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    last_used_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone,
    user_agent text,
    ip_address inet,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_auth_sessions__expires_after_issued CHECK ((expires_at > issued_at)),
    CONSTRAINT ck_auth_sessions__token_hash_length CHECK ((octet_length(token_hash) = 32)),
    CONSTRAINT ck_auth_sessions__user_agent_length CHECK ((char_length(user_agent) <= 512))
);


--
-- Name: auth_sessions_id_seq; Type: SEQUENCE; Schema: identity; Owner: -
--

ALTER TABLE identity.auth_sessions ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME identity.auth_sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_password_auths; Type: TABLE; Schema: identity; Owner: -
--

CREATE TABLE identity.user_password_auths (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    password_hash text NOT NULL,
    algorithm text DEFAULT 'argon2id'::text NOT NULL,
    password_updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    CONSTRAINT ck_user_password_auths__algorithm_allowed CHECK ((algorithm = 'argon2id'::text)),
    CONSTRAINT ck_user_password_auths__password_hash_shape CHECK ((password_hash ~~ '$argon2id$%'::text)),
    CONSTRAINT ck_user_password_auths__version_positive CHECK ((version > 0))
);


--
-- Name: user_password_auths_id_seq; Type: SEQUENCE; Schema: identity; Owner: -
--

ALTER TABLE identity.user_password_auths ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME identity.user_password_auths_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: users; Type: TABLE; Schema: identity; Owner: -
--

CREATE TABLE identity.users (
    id bigint NOT NULL,
    public_id uuid DEFAULT gen_random_uuid() NOT NULL,
    email text NOT NULL,
    display_name text NOT NULL,
    timezone text DEFAULT 'Asia/Tokyo'::text NOT NULL,
    locale text DEFAULT 'ja-JP'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    version integer DEFAULT 1 NOT NULL,
    CONSTRAINT ck_users__display_name_length CHECK (((char_length(display_name) >= 1) AND (char_length(display_name) <= 100))),
    CONSTRAINT ck_users__email_length CHECK (((char_length(email) >= 3) AND (char_length(email) <= 254))),
    CONSTRAINT ck_users__email_lowercase CHECK ((email = lower(email))),
    CONSTRAINT ck_users__email_shape CHECK ((POSITION(('@'::text) IN (email)) > 1)),
    CONSTRAINT ck_users__locale_not_blank CHECK ((btrim(locale) <> ''::text)),
    CONSTRAINT ck_users__timezone_not_blank CHECK ((btrim(timezone) <> ''::text)),
    CONSTRAINT ck_users__version_positive CHECK ((version > 0))
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: identity; Owner: -
--

ALTER TABLE identity.users ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME identity.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: categories; Type: TABLE; Schema: ownership; Owner: -
--

CREATE TABLE ownership.categories (
    id bigint NOT NULL,
    public_id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id bigint NOT NULL,
    name text NOT NULL,
    description text,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    version integer DEFAULT 1 NOT NULL,
    CONSTRAINT ck_categories__description_length CHECK ((char_length(description) <= 500)),
    CONSTRAINT ck_categories__name_length CHECK (((char_length(name) >= 1) AND (char_length(name) <= 100))),
    CONSTRAINT ck_categories__name_not_blank CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT ck_categories__sort_order_not_negative CHECK ((sort_order >= 0)),
    CONSTRAINT ck_categories__version_positive CHECK ((version > 0))
);


--
-- Name: categories_id_seq; Type: SEQUENCE; Schema: ownership; Owner: -
--

ALTER TABLE ownership.categories ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME ownership.categories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: item_tags; Type: TABLE; Schema: ownership; Owner: -
--

CREATE TABLE ownership.item_tags (
    user_id bigint NOT NULL,
    item_id bigint NOT NULL,
    tag_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: items; Type: TABLE; Schema: ownership; Owner: -
--

CREATE TABLE ownership.items (
    id bigint NOT NULL,
    public_id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id bigint NOT NULL,
    category_id bigint NOT NULL,
    name text NOT NULL,
    item_kind_code text NOT NULL,
    quantity integer NOT NULL,
    unit_name text NOT NULL,
    necessity_level_code text NOT NULL,
    usage_frequency_code text NOT NULL,
    purchased_on date,
    source_url text,
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    version integer DEFAULT 1 NOT NULL,
    CONSTRAINT ck_items__item_kind_code_allowed CHECK ((item_kind_code = ANY (ARRAY['durable'::text, 'consumable'::text]))),
    CONSTRAINT ck_items__name_length CHECK (((char_length(name) >= 1) AND (char_length(name) <= 200))),
    CONSTRAINT ck_items__name_not_blank CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT ck_items__necessity_level_code_allowed CHECK ((necessity_level_code = ANY (ARRAY['essential'::text, 'important'::text, 'optional'::text, 'undecided'::text, 'unnecessary'::text]))),
    CONSTRAINT ck_items__notes_length CHECK ((char_length(notes) <= 2000)),
    CONSTRAINT ck_items__quantity_not_negative CHECK ((quantity >= 0)),
    CONSTRAINT ck_items__source_url_length CHECK ((char_length(source_url) <= 2048)),
    CONSTRAINT ck_items__source_url_scheme CHECK (((source_url IS NULL) OR (source_url ~ '^https?://'::text))),
    CONSTRAINT ck_items__unit_name_length CHECK (((char_length(unit_name) >= 1) AND (char_length(unit_name) <= 20))),
    CONSTRAINT ck_items__unit_name_not_blank CHECK ((btrim(unit_name) <> ''::text)),
    CONSTRAINT ck_items__usage_frequency_code_allowed CHECK ((usage_frequency_code = ANY (ARRAY['daily'::text, 'weekly'::text, 'monthly'::text, 'quarterly'::text, 'yearly'::text, 'rarely'::text, 'never'::text]))),
    CONSTRAINT ck_items__version_positive CHECK ((version > 0))
);


--
-- Name: items_id_seq; Type: SEQUENCE; Schema: ownership; Owner: -
--

ALTER TABLE ownership.items ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME ownership.items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: tags; Type: TABLE; Schema: ownership; Owner: -
--

CREATE TABLE ownership.tags (
    id bigint NOT NULL,
    public_id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    version integer DEFAULT 1 NOT NULL,
    CONSTRAINT ck_tags__name_length CHECK (((char_length(name) >= 1) AND (char_length(name) <= 50))),
    CONSTRAINT ck_tags__name_not_blank CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT ck_tags__version_positive CHECK ((version > 0))
);


--
-- Name: tags_id_seq; Type: SEQUENCE; Schema: ownership; Owner: -
--

ALTER TABLE ownership.tags ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME ownership.tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: goose_db_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goose_db_version (
    id integer NOT NULL,
    version_id bigint NOT NULL,
    is_applied boolean NOT NULL,
    tstamp timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: goose_db_version_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.goose_db_version ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.goose_db_version_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: audit_logs pk_audit_logs; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.audit_logs
    ADD CONSTRAINT pk_audit_logs PRIMARY KEY (id);


--
-- Name: audit_logs uq_audit_logs__public_id; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.audit_logs
    ADD CONSTRAINT uq_audit_logs__public_id UNIQUE (public_id);


--
-- Name: auth_sessions pk_auth_sessions; Type: CONSTRAINT; Schema: identity; Owner: -
--

ALTER TABLE ONLY identity.auth_sessions
    ADD CONSTRAINT pk_auth_sessions PRIMARY KEY (id);


--
-- Name: user_password_auths pk_user_password_auths; Type: CONSTRAINT; Schema: identity; Owner: -
--

ALTER TABLE ONLY identity.user_password_auths
    ADD CONSTRAINT pk_user_password_auths PRIMARY KEY (id);


--
-- Name: users pk_users; Type: CONSTRAINT; Schema: identity; Owner: -
--

ALTER TABLE ONLY identity.users
    ADD CONSTRAINT pk_users PRIMARY KEY (id);


--
-- Name: auth_sessions uq_auth_sessions__public_id; Type: CONSTRAINT; Schema: identity; Owner: -
--

ALTER TABLE ONLY identity.auth_sessions
    ADD CONSTRAINT uq_auth_sessions__public_id UNIQUE (public_id);


--
-- Name: auth_sessions uq_auth_sessions__token_hash; Type: CONSTRAINT; Schema: identity; Owner: -
--

ALTER TABLE ONLY identity.auth_sessions
    ADD CONSTRAINT uq_auth_sessions__token_hash UNIQUE (token_hash);


--
-- Name: user_password_auths uq_user_password_auths__user_id; Type: CONSTRAINT; Schema: identity; Owner: -
--

ALTER TABLE ONLY identity.user_password_auths
    ADD CONSTRAINT uq_user_password_auths__user_id UNIQUE (user_id);


--
-- Name: users uq_users__public_id; Type: CONSTRAINT; Schema: identity; Owner: -
--

ALTER TABLE ONLY identity.users
    ADD CONSTRAINT uq_users__public_id UNIQUE (public_id);


--
-- Name: categories pk_categories; Type: CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.categories
    ADD CONSTRAINT pk_categories PRIMARY KEY (id);


--
-- Name: item_tags pk_item_tags; Type: CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.item_tags
    ADD CONSTRAINT pk_item_tags PRIMARY KEY (item_id, tag_id);


--
-- Name: items pk_items; Type: CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.items
    ADD CONSTRAINT pk_items PRIMARY KEY (id);


--
-- Name: tags pk_tags; Type: CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.tags
    ADD CONSTRAINT pk_tags PRIMARY KEY (id);


--
-- Name: categories uq_categories__public_id; Type: CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.categories
    ADD CONSTRAINT uq_categories__public_id UNIQUE (public_id);


--
-- Name: categories uq_categories__user_id_id; Type: CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.categories
    ADD CONSTRAINT uq_categories__user_id_id UNIQUE (user_id, id);


--
-- Name: items uq_items__public_id; Type: CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.items
    ADD CONSTRAINT uq_items__public_id UNIQUE (public_id);


--
-- Name: items uq_items__user_id_id; Type: CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.items
    ADD CONSTRAINT uq_items__user_id_id UNIQUE (user_id, id);


--
-- Name: tags uq_tags__public_id; Type: CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.tags
    ADD CONSTRAINT uq_tags__public_id UNIQUE (public_id);


--
-- Name: tags uq_tags__user_id_id; Type: CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.tags
    ADD CONSTRAINT uq_tags__user_id_id UNIQUE (user_id, id);


--
-- Name: goose_db_version goose_db_version_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goose_db_version
    ADD CONSTRAINT goose_db_version_pkey PRIMARY KEY (id);


--
-- Name: idx_audit_logs__user_id_created_at; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX idx_audit_logs__user_id_created_at ON audit.audit_logs USING btree (user_id, created_at DESC, id DESC);


--
-- Name: idx_auth_sessions__expires_at; Type: INDEX; Schema: identity; Owner: -
--

CREATE INDEX idx_auth_sessions__expires_at ON identity.auth_sessions USING btree (expires_at) WHERE (revoked_at IS NULL);


--
-- Name: idx_auth_sessions__user_id_expires_at; Type: INDEX; Schema: identity; Owner: -
--

CREATE INDEX idx_auth_sessions__user_id_expires_at ON identity.auth_sessions USING btree (user_id, expires_at DESC);


--
-- Name: uq_users__email_active; Type: INDEX; Schema: identity; Owner: -
--

CREATE UNIQUE INDEX uq_users__email_active ON identity.users USING btree (email) WHERE (deleted_at IS NULL);


--
-- Name: idx_categories__user_id_sort_order; Type: INDEX; Schema: ownership; Owner: -
--

CREATE INDEX idx_categories__user_id_sort_order ON ownership.categories USING btree (user_id, sort_order, id) WHERE (deleted_at IS NULL);


--
-- Name: idx_item_tags__tag_id_item_id; Type: INDEX; Schema: ownership; Owner: -
--

CREATE INDEX idx_item_tags__tag_id_item_id ON ownership.item_tags USING btree (tag_id, item_id);


--
-- Name: idx_items__user_id_category_id_deleted_at; Type: INDEX; Schema: ownership; Owner: -
--

CREATE INDEX idx_items__user_id_category_id_deleted_at ON ownership.items USING btree (user_id, category_id, deleted_at);


--
-- Name: idx_items__user_id_deleted_at; Type: INDEX; Schema: ownership; Owner: -
--

CREATE INDEX idx_items__user_id_deleted_at ON ownership.items USING btree (user_id, deleted_at);


--
-- Name: idx_items__user_id_updated_at; Type: INDEX; Schema: ownership; Owner: -
--

CREATE INDEX idx_items__user_id_updated_at ON ownership.items USING btree (user_id, updated_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: idx_tags__user_id_name; Type: INDEX; Schema: ownership; Owner: -
--

CREATE INDEX idx_tags__user_id_name ON ownership.tags USING btree (user_id, name) WHERE (deleted_at IS NULL);


--
-- Name: uq_categories__user_id_name_active; Type: INDEX; Schema: ownership; Owner: -
--

CREATE UNIQUE INDEX uq_categories__user_id_name_active ON ownership.categories USING btree (user_id, name) WHERE (deleted_at IS NULL);


--
-- Name: uq_tags__user_id_name_active; Type: INDEX; Schema: ownership; Owner: -
--

CREATE UNIQUE INDEX uq_tags__user_id_name_active ON ownership.tags USING btree (user_id, name) WHERE (deleted_at IS NULL);


--
-- Name: audit_logs fk_audit_logs__user_id; Type: FK CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.audit_logs
    ADD CONSTRAINT fk_audit_logs__user_id FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE CASCADE;


--
-- Name: auth_sessions fk_auth_sessions__user_id; Type: FK CONSTRAINT; Schema: identity; Owner: -
--

ALTER TABLE ONLY identity.auth_sessions
    ADD CONSTRAINT fk_auth_sessions__user_id FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE CASCADE;


--
-- Name: user_password_auths fk_user_password_auths__user_id; Type: FK CONSTRAINT; Schema: identity; Owner: -
--

ALTER TABLE ONLY identity.user_password_auths
    ADD CONSTRAINT fk_user_password_auths__user_id FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE CASCADE;


--
-- Name: categories fk_categories__user_id; Type: FK CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.categories
    ADD CONSTRAINT fk_categories__user_id FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE CASCADE;


--
-- Name: item_tags fk_item_tags__user_id_item_id; Type: FK CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.item_tags
    ADD CONSTRAINT fk_item_tags__user_id_item_id FOREIGN KEY (user_id, item_id) REFERENCES ownership.items(user_id, id) ON DELETE CASCADE;


--
-- Name: item_tags fk_item_tags__user_id_tag_id; Type: FK CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.item_tags
    ADD CONSTRAINT fk_item_tags__user_id_tag_id FOREIGN KEY (user_id, tag_id) REFERENCES ownership.tags(user_id, id) ON DELETE CASCADE;


--
-- Name: items fk_items__user_id; Type: FK CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.items
    ADD CONSTRAINT fk_items__user_id FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE CASCADE;


--
-- Name: items fk_items__user_id_category_id; Type: FK CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.items
    ADD CONSTRAINT fk_items__user_id_category_id FOREIGN KEY (user_id, category_id) REFERENCES ownership.categories(user_id, id);


--
-- Name: tags fk_tags__user_id; Type: FK CONSTRAINT; Schema: ownership; Owner: -
--

ALTER TABLE ONLY ownership.tags
    ADD CONSTRAINT fk_tags__user_id FOREIGN KEY (user_id) REFERENCES identity.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict 8ocQoXeiE3qiPczVKyjvbJElEk5zdMFTzH0PMTcnZapZBF7gzsD75kXqjdxUON9

