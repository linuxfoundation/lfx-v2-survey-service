-- Copyright The Linux Foundation and each contributor to LFX.
-- SPDX-License-Identifier: MIT

CREATE SEQUENCE IF NOT EXISTS survey_state_revision;
CREATE TABLE IF NOT EXISTS survey_processed_resources (
    resource_kind text NOT NULL CHECK(resource_kind IN ('survey','survey_response','survey_template')),
    source_id text NOT NULL,
    state text NOT NULL CHECK(state IN ('empty','processed','deleted','legacy')),
    legacy_uid text NOT NULL DEFAULT '',
    version bigint NOT NULL DEFAULT nextval('survey_state_revision'),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(resource_kind,source_id)
);
CREATE TABLE IF NOT EXISTS survey_invitation_guards (
    response_id text PRIMARY KEY,
    state text NOT NULL CHECK(state IN ('empty','pending','sent','deleted')),
    invite_id text NOT NULL DEFAULT '',
    version bigint NOT NULL DEFAULT nextval('survey_state_revision'),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS survey_invitation_intents (
    response_id text PRIMARY KEY,
    resolved boolean NOT NULL DEFAULT false
);
ALTER TABLE survey_invitation_intents ADD COLUMN IF NOT EXISTS resolved boolean NOT NULL DEFAULT false;
CREATE TABLE IF NOT EXISTS survey_state_cutover (
    singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
    source_last_sequence bigint NOT NULL,
    verified_at timestamptz NOT NULL DEFAULT now()
);
