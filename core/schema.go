package core

var files = map[string]string{
	"schema.sql": "--\n-- PostgreSQL database dump\n--\n\n-- Dumped from database version 9.5.0\n-- Dumped by pg_dump version 9.5.0\n\nSET statement_timeout = 0;\nSET lock_timeout = 0;\nSET client_encoding = 'UTF8';\nSET standard_conforming_strings = on;\nSET check_function_bodies = false;\nSET client_min_messages = warning;\nSET row_security = off;\n\n--\n-- Name: plpgsql; Type: EXTENSION; Schema: -; Owner: -\n--\n\nCREATE EXTENSION IF NOT EXISTS plpgsql WITH SCHEMA pg_catalog;\n\n\n--\n--\n\n\n\nSET search_path = public, pg_catalog;\n\n--\n-- Name: access_token_type; Type: TYPE; Schema: public; Owner: -\n--\n\nCREATE TYPE access_token_type AS ENUM (\n    'client',\n    'network'\n);\n\n\n--\n-- Name: b32enc_crockford(bytea); Type: FUNCTION; Schema: public; Owner: -\n--\n\nCREATE FUNCTION b32enc_crockford(src bytea) RETURNS text\n    LANGUAGE plpgsql IMMUTABLE\n    AS $$\n\t-- Adapted from the Go package encoding/base32.\n\t-- See https://golang.org/src/encoding/base32/base32.go.\n\t-- NOTE(kr): this function does not pad its output\nDECLARE\n\t-- alphabet is the base32 alphabet defined\n\t-- by Douglas Crockford. It preserves lexical\n\t-- order and avoids visually-similar symbols.\n\t-- See http://www.crockford.com/wrmg/base32.html.\n\talphabet text := '0123456789ABCDEFGHJKMNPQRSTVWXYZ';\n\tdst text := '';\n\tn integer;\n\tb0 integer;\n\tb1 integer;\n\tb2 integer;\n\tb3 integer;\n\tb4 integer;\n\tb5 integer;\n\tb6 integer;\n\tb7 integer;\nBEGIN\n\tFOR r IN 0..(length(src)-1) BY 5\n\tLOOP\n\t\tb0:=0; b1:=0; b2:=0; b3:=0; b4:=0; b5:=0; b6:=0; b7:=0;\n\n\t\t-- Unpack 8x 5-bit source blocks into an 8 byte\n\t\t-- destination quantum\n\t\tn := length(src) - r;\n\t\tIF n >= 5 THEN\n\t\t\tb7 := get_byte(src, r+4) & 31;\n\t\t\tb6 := get_byte(src, r+4) >> 5;\n\t\tEND IF;\n\t\tIF n >= 4 THEN\n\t\t\tb6 := b6 | (get_byte(src, r+3) << 3) & 31;\n\t\t\tb5 := (get_byte(src, r+3) >> 2) & 31;\n\t\t\tb4 := get_byte(src, r+3) >> 7;\n\t\tEND IF;\n\t\tIF n >= 3 THEN\n\t\t\tb4 := b4 | (get_byte(src, r+2) << 1) & 31;\n\t\t\tb3 := (get_byte(src, r+2) >> 4) & 31;\n\t\tEND IF;\n\t\tIF n >= 2 THEN\n\t\t\tb3 := b3 | (get_byte(src, r+1) << 4) & 31;\n\t\t\tb2 := (get_byte(src, r+1) >> 1) & 31;\n\t\t\tb1 := (get_byte(src, r+1) >> 6) & 31;\n\t\tEND IF;\n\t\tb1 := b1 | (get_byte(src, r) << 2) & 31;\n\t\tb0 := get_byte(src, r) >> 3;\n\n\t\t-- Encode 5-bit blocks using the base32 alphabet\n\t\tdst := dst || substr(alphabet, b0+1, 1);\n\t\tdst := dst || substr(alphabet, b1+1, 1);\n\t\tIF n >= 2 THEN\n\t\t\tdst := dst || substr(alphabet, b2+1, 1);\n\t\t\tdst := dst || substr(alphabet, b3+1, 1);\n\t\tEND IF;\n\t\tIF n >= 3 THEN\n\t\t\tdst := dst || substr(alphabet, b4+1, 1);\n\t\tEND IF;\n\t\tIF n >= 4 THEN\n\t\t\tdst := dst || substr(alphabet, b5+1, 1);\n\t\t\tdst := dst || substr(alphabet, b6+1, 1);\n\t\tEND IF;\n\t\tIF n >= 5 THEN\n\t\t\tdst := dst || substr(alphabet, b7+1, 1);\n\t\tEND IF;\n\tEND LOOP;\n\tRETURN dst;\nEND;\n$$;\n\n\n--\n-- Name: cancel_reservation(integer); Type: FUNCTION; Schema: public; Owner: -\n--\n\nCREATE FUNCTION cancel_reservation(inp_reservation_id integer) RETURNS void\n    LANGUAGE plpgsql\n    AS $$\nBEGIN\n    DELETE FROM reservations WHERE reservation_id = inp_reservation_id;\nEND;\n$$;\n\n\n--\n-- Name: create_reservation(text, text, timestamp with time zone, text); Type: FUNCTION; Schema: public; Owner: -\n--\n\nCREATE FUNCTION create_reservation(inp_asset_id text, inp_account_id text, inp_expiry timestamp with time zone, inp_idempotency_key text, OUT reservation_id integer, OUT already_existed boolean, OUT existing_change bigint) RETURNS record\n    LANGUAGE plpgsql\n    AS $$\nDECLARE\n    row RECORD;\nBEGIN\n    INSERT INTO reservations (asset_id, account_id, expiry, idempotency_key)\n        VALUES (inp_asset_id, inp_account_id, inp_expiry, inp_idempotency_key)\n        ON CONFLICT (idempotency_key) DO NOTHING\n        RETURNING reservations.reservation_id, FALSE AS already_existed, CAST(0 AS BIGINT) AS existing_change INTO row;\n    -- Iff the insert was successful, then a row is returned. The IF NOT FOUND check\n    -- will be true iff the insert failed because the row already exists.\n    IF NOT FOUND THEN\n        SELECT r.reservation_id, TRUE AS already_existed, r.change AS existing_change INTO STRICT row\n            FROM reservations r\n            WHERE r.idempotency_key = inp_idempotency_key;\n    END IF;\n    reservation_id := row.reservation_id;\n    already_existed := row.already_existed;\n    existing_change := row.existing_change;\nEND;\n$$;\n\n\n--\n-- Name: expire_reservations(); Type: FUNCTION; Schema: public; Owner: -\n--\n\nCREATE FUNCTION expire_reservations() RETURNS void\n    LANGUAGE plpgsql\n    AS $$\nBEGIN\n    DELETE FROM reservations WHERE expiry < CURRENT_TIMESTAMP;\nEND;\n$$;\n\n\n--\n-- Name: next_chain_id(text); Type: FUNCTION; Schema: public; Owner: -\n--\n\nCREATE FUNCTION next_chain_id(prefix text) RETURNS text\n    LANGUAGE plpgsql\n    AS $$\n\t-- Adapted from the technique published by Instagram.\n\t-- See http://instagram-engineering.tumblr.com/post/10853187575/sharding-ids-at-instagram.\nDECLARE\n\tour_epoch_ms bigint := 1433333333333; -- do not change\n\tseq_id bigint;\n\tnow_ms bigint;     -- from unix epoch, not ours\n\tshard_id int := 4; -- must be different on each shard\n\tn bigint;\nBEGIN\n\tSELECT nextval('chain_id_seq') % 1024 INTO seq_id;\n\tSELECT FLOOR(EXTRACT(EPOCH FROM clock_timestamp()) * 1000) INTO now_ms;\n\tn := (now_ms - our_epoch_ms) << 23;\n\tn := n | (shard_id << 10);\n\tn := n | (seq_id);\n\tRETURN prefix || b32enc_crockford(int8send(n));\nEND;\n$$;\n\n\n--\n-- Name: reserve_utxo(text, bigint, timestamp with time zone, text); Type: FUNCTION; Schema: public; Owner: -\n--\n\nCREATE FUNCTION reserve_utxo(inp_tx_hash text, inp_out_index bigint, inp_expiry timestamp with time zone, inp_idempotency_key text) RETURNS record\n    LANGUAGE plpgsql\n    AS $$\nDECLARE\n    res RECORD;\n    row RECORD;\n    ret RECORD;\nBEGIN\n    SELECT * FROM create_reservation(NULL, NULL, inp_expiry, inp_idempotency_key) INTO STRICT res;\n    IF res.already_existed THEN\n      SELECT res.reservation_id, res.already_existed, res.existing_change, CAST(0 AS BIGINT) AS amount, FALSE AS insufficient INTO ret;\n      RETURN ret;\n    END IF;\n\n    SELECT tx_hash, index, amount INTO row\n        FROM account_utxos u\n        WHERE inp_tx_hash = tx_hash\n              AND inp_out_index = index\n              AND reservation_id IS NULL\n        LIMIT 1\n        FOR UPDATE\n        SKIP LOCKED;\n    IF FOUND THEN\n        UPDATE account_utxos SET reservation_id = res.reservation_id\n            WHERE (tx_hash, index) = (row.tx_hash, row.index);\n    ELSE\n      PERFORM cancel_reservation(res.reservation_id);\n      res.reservation_id := 0;\n    END IF;\n\n    SELECT res.reservation_id, res.already_existed, EXISTS(SELECT tx_hash FROM account_utxos WHERE tx_hash = inp_tx_hash AND index = inp_out_index) INTO ret;\n    RETURN ret;\nEND;\n$$;\n\n\n--\n-- Name: reserve_utxos(text, text, text, bigint, bigint, timestamp with time zone, text); Type: FUNCTION; Schema: public; Owner: -\n--\n\nCREATE FUNCTION reserve_utxos(inp_asset_id text, inp_account_id text, inp_tx_hash text, inp_out_index bigint, inp_amt bigint, inp_expiry timestamp with time zone, inp_idempotency_key text) RETURNS record\n    LANGUAGE plpgsql\n    AS $$\nDECLARE\n    res RECORD;\n    row RECORD;\n    ret RECORD;\n    available BIGINT := 0;\n    unavailable BIGINT := 0;\nBEGIN\n    SELECT * FROM create_reservation(inp_asset_id, inp_account_id, inp_expiry, inp_idempotency_key) INTO STRICT res;\n    IF res.already_existed THEN\n      SELECT res.reservation_id, res.already_existed, res.existing_change, CAST(0 AS BIGINT) AS amount, FALSE AS insufficient INTO ret;\n      RETURN ret;\n    END IF;\n\n    LOOP\n        SELECT tx_hash, index, amount INTO row\n            FROM account_utxos u\n            WHERE asset_id = inp_asset_id\n                  AND inp_account_id = account_id\n                  AND (inp_tx_hash IS NULL OR inp_tx_hash = tx_hash)\n                  AND (inp_out_index IS NULL OR inp_out_index = index)\n                  AND reservation_id IS NULL\n            LIMIT 1\n            FOR UPDATE\n            SKIP LOCKED;\n        IF FOUND THEN\n            UPDATE account_utxos SET reservation_id = res.reservation_id\n                WHERE (tx_hash, index) = (row.tx_hash, row.index);\n            available := available + row.amount;\n            IF available >= inp_amt THEN\n                EXIT;\n            END IF;\n        ELSE\n            EXIT;\n        END IF;\n    END LOOP;\n\n    IF available < inp_amt THEN\n        SELECT SUM(change) AS change INTO STRICT row\n            FROM reservations\n            WHERE asset_id = inp_asset_id AND account_id = inp_account_id;\n        unavailable := row.change;\n        PERFORM cancel_reservation(res.reservation_id);\n        res.reservation_id := 0;\n    ELSE\n        UPDATE reservations SET change = available - inp_amt\n            WHERE reservation_id = res.reservation_id;\n    END IF;\n\n    SELECT res.reservation_id, res.already_existed, CAST(0 AS BIGINT) AS existing_change, available AS amount, (available+unavailable < inp_amt) AS insufficient INTO ret;\n    RETURN ret;\nEND;\n$$;\n\n\nSET default_tablespace = '';\n\nSET default_with_oids = false;\n\n--\n-- Name: access_tokens; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE access_tokens (\n    id text NOT NULL,\n    sort_id text DEFAULT next_chain_id('at'::text),\n    type access_token_type NOT NULL,\n    hashed_secret bytea NOT NULL,\n    created timestamp with time zone DEFAULT now() NOT NULL\n);\n\n\n--\n-- Name: account_control_program_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE account_control_program_seq\n    START WITH 10001\n    INCREMENT BY 10000\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: account_control_programs; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE account_control_programs (\n    id text DEFAULT next_chain_id('acp'::text) NOT NULL,\n    signer_id text NOT NULL,\n    key_index bigint NOT NULL,\n    control_program bytea NOT NULL,\n    change boolean NOT NULL\n);\n\n\n--\n-- Name: account_utxos; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE account_utxos (\n    tx_hash text NOT NULL,\n    index integer NOT NULL,\n    asset_id text NOT NULL,\n    amount bigint NOT NULL,\n    account_id text NOT NULL,\n    control_program_index bigint NOT NULL,\n    reservation_id integer,\n    control_program bytea NOT NULL,\n    metadata bytea NOT NULL,\n    confirmed_in bigint,\n    block_pos integer,\n    block_timestamp bigint,\n    expiry_height bigint\n);\n\n\n--\n-- Name: accounts; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE accounts (\n    account_id text NOT NULL,\n    tags jsonb,\n    alias text\n);\n\n\n--\n-- Name: annotated_accounts; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE annotated_accounts (\n    id text NOT NULL,\n    data jsonb NOT NULL\n);\n\n\n--\n-- Name: annotated_assets; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE annotated_assets (\n    id text NOT NULL,\n    data jsonb NOT NULL,\n    sort_id text NOT NULL\n);\n\n\n--\n-- Name: annotated_outputs; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE annotated_outputs (\n    block_height bigint NOT NULL,\n    tx_pos integer NOT NULL,\n    output_index integer NOT NULL,\n    tx_hash text NOT NULL,\n    data jsonb NOT NULL,\n    timespan int8range NOT NULL\n);\n\n\n--\n-- Name: annotated_txs; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE annotated_txs (\n    block_height bigint NOT NULL,\n    tx_pos integer NOT NULL,\n    tx_hash text NOT NULL,\n    data jsonb NOT NULL\n);\n\n\n--\n-- Name: asset_tags; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE asset_tags (\n    asset_id text NOT NULL,\n    tags jsonb\n);\n\n\n--\n-- Name: assets; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE assets (\n    id text NOT NULL,\n    created_at timestamp with time zone DEFAULT now() NOT NULL,\n    definition_mutable boolean DEFAULT false NOT NULL,\n    sort_id text DEFAULT next_chain_id('asset'::text) NOT NULL,\n    issuance_program bytea NOT NULL,\n    client_token text,\n    initial_block_hash text NOT NULL,\n    signer_id text,\n    definition jsonb,\n    alias text,\n    first_block_height bigint\n);\n\n\n--\n-- Name: assets_key_index_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE assets_key_index_seq\n    START WITH 1\n    INCREMENT BY 1\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: blocks; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE blocks (\n    block_hash text NOT NULL,\n    height bigint NOT NULL,\n    data bytea NOT NULL,\n    header bytea NOT NULL\n);\n\n\n--\n-- Name: chain_id_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE chain_id_seq\n    START WITH 1\n    INCREMENT BY 1\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: config; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE config (\n    singleton boolean DEFAULT true NOT NULL,\n    is_signer boolean,\n    is_generator boolean,\n    blockchain_id text NOT NULL,\n    configured_at timestamp with time zone NOT NULL,\n    generator_url text DEFAULT ''::text NOT NULL,\n    block_xpub text DEFAULT ''::text NOT NULL,\n    remote_block_signers bytea DEFAULT '\\x'::bytea NOT NULL,\n    generator_access_token text DEFAULT ''::text NOT NULL,\n    CONSTRAINT config_singleton CHECK (singleton)\n);\n\n\n--\n-- Name: generator_pending_block; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE generator_pending_block (\n    singleton boolean DEFAULT true NOT NULL,\n    data bytea NOT NULL,\n    CONSTRAINT generator_pending_block_singleton CHECK (singleton)\n);\n\n\n--\n-- Name: leader; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE leader (\n    singleton boolean DEFAULT true NOT NULL,\n    leader_key text NOT NULL,\n    expiry timestamp with time zone DEFAULT '1970-01-01 00:00:00-08'::timestamp with time zone NOT NULL,\n    address text NOT NULL,\n    CONSTRAINT leader_singleton CHECK (singleton)\n);\n\n\n--\n-- Name: migrations; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE migrations (\n    filename text NOT NULL,\n    hash text NOT NULL,\n    applied_at timestamp with time zone DEFAULT now() NOT NULL\n);\n\n\n--\n-- Name: mockhsm_sort_id_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE mockhsm_sort_id_seq\n    START WITH 1\n    INCREMENT BY 1\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: mockhsm; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE mockhsm (\n    xpub bytea NOT NULL,\n    xprv bytea NOT NULL,\n    xpub_hash text NOT NULL,\n    alias text,\n    sort_id bigint DEFAULT nextval('mockhsm_sort_id_seq'::regclass) NOT NULL\n);\n\n\n--\n-- Name: pool_tx_sort_id_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE pool_tx_sort_id_seq\n    START WITH 1\n    INCREMENT BY 1\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: pool_txs; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE UNLOGGED TABLE pool_txs (\n    tx_hash text NOT NULL,\n    data bytea NOT NULL,\n    sort_id bigint DEFAULT nextval('pool_tx_sort_id_seq'::regclass) NOT NULL\n);\n\n\n--\n-- Name: query_blocks; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE query_blocks (\n    height bigint NOT NULL,\n    \"timestamp\" bigint NOT NULL\n);\n\n\n--\n-- Name: reservation_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE reservation_seq\n    START WITH 1\n    INCREMENT BY 1\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: reservations; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE reservations (\n    reservation_id integer DEFAULT nextval('reservation_seq'::regclass) NOT NULL,\n    asset_id text,\n    account_id text,\n    expiry timestamp with time zone DEFAULT '1970-01-01 00:00:00-08'::timestamp with time zone NOT NULL,\n    change bigint DEFAULT 0 NOT NULL,\n    idempotency_key text\n);\n\n\n--\n-- Name: signed_blocks; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE signed_blocks (\n    block_height bigint NOT NULL,\n    block_hash text NOT NULL\n);\n\n\n--\n-- Name: signers; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE signers (\n    id text NOT NULL,\n    type text NOT NULL,\n    key_index bigint NOT NULL,\n    xpubs text[] NOT NULL,\n    quorum integer NOT NULL,\n    client_token text\n);\n\n\n--\n-- Name: signers_key_index_seq; Type: SEQUENCE; Schema: public; Owner: -\n--\n\nCREATE SEQUENCE signers_key_index_seq\n    START WITH 1\n    INCREMENT BY 1\n    NO MINVALUE\n    NO MAXVALUE\n    CACHE 1;\n\n\n--\n-- Name: signers_key_index_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -\n--\n\nALTER SEQUENCE signers_key_index_seq OWNED BY signers.key_index;\n\n\n--\n-- Name: snapshots; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE snapshots (\n    height bigint NOT NULL,\n    data bytea NOT NULL\n);\n\n\n--\n-- Name: submitted_txs; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE submitted_txs (\n    tx_id text NOT NULL,\n    height bigint NOT NULL,\n    submitted_at timestamp without time zone DEFAULT now() NOT NULL\n);\n\n\n--\n-- Name: txconsumers; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE txconsumers (\n    id text DEFAULT next_chain_id('cur'::text) NOT NULL,\n    alias text,\n    filter text,\n    after text,\n    client_token text NOT NULL\n);\n\n\n--\n-- Name: key_index; Type: DEFAULT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY signers ALTER COLUMN key_index SET DEFAULT nextval('signers_key_index_seq'::regclass);\n\n\n--\n-- Name: access_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY access_tokens\n    ADD CONSTRAINT access_tokens_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: account_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY accounts\n    ADD CONSTRAINT account_tags_pkey PRIMARY KEY (account_id);\n\n\n--\n-- Name: account_utxos_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY account_utxos\n    ADD CONSTRAINT account_utxos_pkey PRIMARY KEY (tx_hash, index);\n\n\n--\n-- Name: accounts_alias_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY accounts\n    ADD CONSTRAINT accounts_alias_key UNIQUE (alias);\n\n\n--\n-- Name: annotated_accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY annotated_accounts\n    ADD CONSTRAINT annotated_accounts_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: annotated_assets_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY annotated_assets\n    ADD CONSTRAINT annotated_assets_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: annotated_outputs_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY annotated_outputs\n    ADD CONSTRAINT annotated_outputs_pkey PRIMARY KEY (block_height, tx_pos, output_index);\n\n\n--\n-- Name: annotated_txs_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY annotated_txs\n    ADD CONSTRAINT annotated_txs_pkey PRIMARY KEY (block_height, tx_pos);\n\n\n--\n-- Name: asset_tags_asset_id_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY asset_tags\n    ADD CONSTRAINT asset_tags_asset_id_key UNIQUE (asset_id);\n\n\n--\n-- Name: assets_alias_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY assets\n    ADD CONSTRAINT assets_alias_key UNIQUE (alias);\n\n\n--\n-- Name: assets_client_token_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY assets\n    ADD CONSTRAINT assets_client_token_key UNIQUE (client_token);\n\n\n--\n-- Name: assets_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY assets\n    ADD CONSTRAINT assets_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: blocks_height_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY blocks\n    ADD CONSTRAINT blocks_height_key UNIQUE (height);\n\n\n--\n-- Name: blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY blocks\n    ADD CONSTRAINT blocks_pkey PRIMARY KEY (block_hash);\n\n\n--\n-- Name: config_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY config\n    ADD CONSTRAINT config_pkey PRIMARY KEY (singleton);\n\n\n--\n-- Name: generator_pending_block_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY generator_pending_block\n    ADD CONSTRAINT generator_pending_block_pkey PRIMARY KEY (singleton);\n\n\n--\n-- Name: leader_singleton_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY leader\n    ADD CONSTRAINT leader_singleton_key UNIQUE (singleton);\n\n\n--\n-- Name: migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY migrations\n    ADD CONSTRAINT migrations_pkey PRIMARY KEY (filename);\n\n\n--\n-- Name: mockhsm_alias_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY mockhsm\n    ADD CONSTRAINT mockhsm_alias_key UNIQUE (alias);\n\n\n--\n-- Name: mockhsm_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY mockhsm\n    ADD CONSTRAINT mockhsm_pkey PRIMARY KEY (xpub);\n\n\n--\n-- Name: pool_txs_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY pool_txs\n    ADD CONSTRAINT pool_txs_pkey PRIMARY KEY (tx_hash);\n\n\n--\n-- Name: pool_txs_sort_id_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY pool_txs\n    ADD CONSTRAINT pool_txs_sort_id_key UNIQUE (sort_id);\n\n\n--\n-- Name: query_blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY query_blocks\n    ADD CONSTRAINT query_blocks_pkey PRIMARY KEY (height);\n\n\n--\n-- Name: reservations_idempotency_key_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY reservations\n    ADD CONSTRAINT reservations_idempotency_key_key UNIQUE (idempotency_key);\n\n\n--\n-- Name: reservations_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY reservations\n    ADD CONSTRAINT reservations_pkey PRIMARY KEY (reservation_id);\n\n\n--\n-- Name: signers_client_token_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY signers\n    ADD CONSTRAINT signers_client_token_key UNIQUE (client_token);\n\n\n--\n-- Name: signers_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY signers\n    ADD CONSTRAINT signers_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: sort_id_index; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY mockhsm\n    ADD CONSTRAINT sort_id_index UNIQUE (sort_id);\n\n\n--\n-- Name: state_trees_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY snapshots\n    ADD CONSTRAINT state_trees_pkey PRIMARY KEY (height);\n\n\n--\n-- Name: submitted_txs_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY submitted_txs\n    ADD CONSTRAINT submitted_txs_pkey PRIMARY KEY (tx_id);\n\n\n--\n-- Name: txconsumers_alias_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY txconsumers\n    ADD CONSTRAINT txconsumers_alias_key UNIQUE (alias);\n\n\n--\n-- Name: txconsumers_client_token_key; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY txconsumers\n    ADD CONSTRAINT txconsumers_client_token_key UNIQUE (client_token);\n\n\n--\n-- Name: txconsumers_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY txconsumers\n    ADD CONSTRAINT txconsumers_pkey PRIMARY KEY (id);\n\n\n--\n-- Name: account_control_programs_control_program_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX account_control_programs_control_program_idx ON account_control_programs USING btree (control_program);\n\n\n--\n-- Name: account_utxos_account_id; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX account_utxos_account_id ON account_utxos USING btree (account_id);\n\n\n--\n-- Name: account_utxos_account_id_asset_id_tx_hash_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX account_utxos_account_id_asset_id_tx_hash_idx ON account_utxos USING btree (account_id, asset_id, tx_hash);\n\n\n--\n-- Name: account_utxos_expiry_height_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX account_utxos_expiry_height_idx ON account_utxos USING btree (expiry_height) WHERE (confirmed_in IS NULL);\n\n\n--\n-- Name: account_utxos_reservation_id_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX account_utxos_reservation_id_idx ON account_utxos USING btree (reservation_id);\n\n\n--\n-- Name: annotated_accounts_jsondata_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX annotated_accounts_jsondata_idx ON annotated_accounts USING gin (data jsonb_path_ops);\n\n\n--\n-- Name: annotated_assets_jsondata_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX annotated_assets_jsondata_idx ON annotated_assets USING gin (data jsonb_path_ops);\n\n\n--\n-- Name: annotated_assets_sort_id; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX annotated_assets_sort_id ON annotated_assets USING btree (sort_id);\n\n\n--\n-- Name: annotated_outputs_jsondata_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX annotated_outputs_jsondata_idx ON annotated_outputs USING gin (data jsonb_path_ops);\n\n\n--\n-- Name: annotated_outputs_outpoint_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX annotated_outputs_outpoint_idx ON annotated_outputs USING btree (tx_hash, output_index);\n\n\n--\n-- Name: annotated_outputs_timespan_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX annotated_outputs_timespan_idx ON annotated_outputs USING gist (timespan);\n\n\n--\n-- Name: annotated_txs_data; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX annotated_txs_data ON annotated_txs USING gin (data);\n\n\n--\n-- Name: assets_sort_id; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX assets_sort_id ON assets USING btree (sort_id);\n\n\n--\n-- Name: query_blocks_timestamp_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX query_blocks_timestamp_idx ON query_blocks USING btree (\"timestamp\");\n\n\n--\n-- Name: reservations_asset_id_account_id_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX reservations_asset_id_account_id_idx ON reservations USING btree (asset_id, account_id);\n\n\n--\n-- Name: reservations_expiry; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX reservations_expiry ON reservations USING btree (expiry);\n\n\n--\n-- Name: signed_blocks_block_height_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE UNIQUE INDEX signed_blocks_block_height_idx ON signed_blocks USING btree (block_height);\n\n\n--\n-- Name: signers_type_id_idx; Type: INDEX; Schema: public; Owner: -\n--\n\nCREATE INDEX signers_type_id_idx ON signers USING btree (type, id);\n\n\n--\n-- Name: account_utxos_reservation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY account_utxos\n    ADD CONSTRAINT account_utxos_reservation_id_fkey FOREIGN KEY (reservation_id) REFERENCES reservations(reservation_id) ON DELETE SET NULL;\n\n\n--\n-- PostgreSQL database dump complete\n--\n\ninsert into migrations (filename, hash) values ('2016-08-16.0.core.initial-schema.sql', 'ee7466ce9642af3afa60c3104d3d0f8f4161e4a9f58b1daa58e005a6566b82e2');\ninsert into migrations (filename, hash) values ('2016-08-16.1.api.add-asset-aliases.sql', '78b8c814db73872e6ebc8c5bcedc342d17f566a0a637470aed2761ae09060873');\ninsert into migrations (filename, hash) values ('2016-08-16.2.query.rename-index-alias.sql', 'eb1b6b4fa602ac21d7f957f8fda0b94dd50fb710055842cead111a0bb35d93ae');\ninsert into migrations (filename, hash) values ('2016-08-16.3.api.alias-keys.sql', 'ee7702a963064b004800ee356558ec2a2a3062f443ff7871d1fc2a873f22665e');\ninsert into migrations (filename, hash) values ('2016-08-17.0.query.index-id.sql', '538ce1a1f61b496d1809049f3934ba445177e5b71af2e802d2fbcb009a8d80cb');\ninsert into migrations (filename, hash) values ('2016-08-19.0.generator.generated-block.sql', '8068324f63c2d973f0eac120460ca202711bfab0f734b789f18206f21abd3a80');\ninsert into migrations (filename, hash) values ('2016-08-22.0.account.change-utxos.sql', '11dff1da7353fd6896c4f153654029d717f838b4a4e528cf7139bfcf35ebf124');\ninsert into migrations (filename, hash) values ('2016-08-23.0.query.sum-by.sql', '82a04d0595f19df735dd35e896496c79a70398a8428131ece4d441aaf2f3836c');\ninsert into migrations (filename, hash) values ('2016-08-24.0.query.index-filter.sql', '9b501c1fc5a528312f7239a58b1552bf95681fe228d57472a65e9a55fc19246b');\ninsert into migrations (filename, hash) values ('2016-08-26.0.query.assets-sort-id.sql', '56083fef381b675be65ef9c9769de724ddf46f2c3ed44a9e75ab81ecc812f983');\ninsert into migrations (filename, hash) values ('2016-08-29.0.core.config.sql', '4f440fccb3a8523bfd4455acf400e20859d134118742a55677a04d2297297914');\ninsert into migrations (filename, hash) values ('2016-08-30.0.asset.issuance-totals.sql', '2a4b3f9899df7c099eb215c0bf5b6b7e2ac6f991c08820c000a96ddf8cfb2671');\ninsert into migrations (filename, hash) values ('2016-08-31.0.core.add-leader-address.sql', '6d2f4eca68067afae4531b8e56de4c7628158e2dd66cd1831585d26e83817f1b');\ninsert into migrations (filename, hash) values ('2016-08-31.1.core.mockhsm-key-sort.sql', '4ecebb1e4485e6ea7b0fb7ed3a34f5c4f511fdec0379d67fcb646ec06708ad70');\ninsert into migrations (filename, hash) values ('2016-08-31.2.core.default-config-generator-url.sql', '4b3a62ed2ff07256b6289b1f0e02910d9ddf5e345c169a978910306cad1c6948');\ninsert into migrations (filename, hash) values ('2016-08-31.3.account.drop-redeem-program.sql', '8b1dfd7056ba04cbb2609a9fa0e8f8f76e6fca3d19b8c2a3cbd88f0513022462');\ninsert into migrations (filename, hash) values ('2016-09-01.0.core.add-block-xpub.sql', '89051edcfbfba56cc09870ab9864b8605babded501a612e0a3882823b4ebbdba');\ninsert into migrations (filename, hash) values ('2016-09-01.1.core.rename-genesis-to-initial.sql', '25699037b44a16db6e3fdbfe81f35dea5dbf4b230ff3f63904f5150f08955208');\ninsert into migrations (filename, hash) values ('2016-09-01.2.core.drop-txs.sql', '14e21eba20efe97745d7e0a1a17e51582e2c05597030db1a3a01d46258ff2574');\ninsert into migrations (filename, hash) values ('2016-09-05.0.asset.signer-null.sql', 'e611da44df43ea431c937c77c1e852fe82dc0049a118ec896102ca8a2cfb09f6');\ninsert into migrations (filename, hash) values ('2016-09-06.0.asset.height.sql', 'd63ac300dfdaaa9ea48741b5d9c66af27a11eba46621ee2cb9a76ba39b2e50a6');\ninsert into migrations (filename, hash) values ('2016-09-14.0.appdb.add-cursors-table.sql', 'fedda13d07e22fce61508dfdc38cafba9492439200e0d120c7438dc7489f4c3a');\ninsert into migrations (filename, hash) values ('2016-09-14.1.query.remove-indexes.sql', '47f772881706b2a518f79f22a7dd82512ce3994ba2b97cd7cfc25272aadf8f32');\ninsert into migrations (filename, hash) values ('2016-09-14.2.appdb.add-cursors-id.sql', '6125ca2a8c73131e2e619d8edfe59518423482929e13df1f98a24d012cf453bb');\ninsert into migrations (filename, hash) values ('2016-09-23.0.account.change-control-programs.sql', 'dd5fe8c4b418c061bea8007b61dfefb64418f9a99c2978f559b4628ec323bc25');\ninsert into migrations (filename, hash) values ('2016-09-26.0.core.add-access-tokens.sql', '81b12b6aed53dfacca8f8d922b377ef24c727950dea2c8c186d6f68400de511a');\ninsert into migrations (filename, hash) values ('2016-09-26.1.core.add-require-auth-flags.sql', '0e21f6d4836fcde1bdec80b0f6cc7c8cf8355fee47a7d5561ed8d7f6726425f1');\ninsert into migrations (filename, hash) values ('2016-09-29.0.core.drop-archived-columns.sql', '557b8ae9b6604485d7ae7eef2206ce218d4bb3a1ac9bcdac6b4691db1da20208');\ninsert into migrations (filename, hash) values ('2016-09-29.1.core.cursors-to-txconsumers.sql', 'ed2fee4f5e726ba76eff4c7e3aaceac7e960164031df7f89e2a33b5de4e25216');\ninsert into migrations (filename, hash) values ('2016-09-29.2.core.rename-blockchain-id.sql', 'bc1dab19322441951dcc2a49577f1b477cca8ae2e9b6c6568ba57028606b64fc');\ninsert into migrations (filename, hash) values ('2016-09-29.3.signers.remove-key-index.sql', '797977c67310d658496138a3d956f9d34c52a4462ab55a5754ecaa8b35908904');\ninsert into migrations (filename, hash) values ('2016-09-30.0.core.remove-is-ascending-from-txconsumers.sql', 'dc22181470cd84be345701616755371e4612457955f460110ab0f88aeaa85222');\ninsert into migrations (filename, hash) values ('2016-09-30.1.core.submit-idempotence.sql', '2dd217fda2f33f72332d0502149c1b1a75e5e896b5252477a35a7f62bb49052f');\ninsert into migrations (filename, hash) values ('2016-09-30.2.config.add-secrets-signers.sql', 'da6c7ee122069bbbed470407f4090e364916d8d54339177b21afbadd820bec0e');\ninsert into migrations (filename, hash) values ('2016-10-03.1.config.add-generater-token.sql', 'f1f7e6ea6fcbd773242954d32cc402173913396c8c1233fb573ab8a2e5d770f9');\ninsert into migrations (filename, hash) values ('2016-10-04.0.txdb.pool-txs-unlogged.sql', '5baa0d1e2890d8be09c9defc3dc0c81ab9fb012cabf000fffc8330eee0d5708a');\ninsert into migrations (filename, hash) values ('2016-10-05.0.core.remote-require-auth-flags.sql', '3dad64f0e38c8140d471b851e3ba5bb5045bf5e760488e0ab3b617ddb09c62cf');\ninsert into migrations (filename, hash) values ('2016-10-05.1.core.add-testnet-config.sql', '6e0c8da9c9adb3f85f7d4baee00f84b3b4183c8ebad071d54a0128d8b414b007');\ninsert into migrations (filename, hash) values ('2016-10-05.2.core.remove-testnet-config.sql', 'f748d503f055882d3c5af641931c3266a120d7b3ba404424db78a1f075f76b95');\ninsert into migrations (filename, hash) values ('2016-10-05.3.account.index-expiry-height.sql', 'a1db582321307ad8fc49c201e95f7380f5f8441d6ab9a11838433666441817d3');\n",
}
