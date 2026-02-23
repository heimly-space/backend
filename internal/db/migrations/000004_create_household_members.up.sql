CREATE TABLE household_members
(
    id           UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    household_id UUID        NOT NULL,
    user_id      UUID        NOT NULL,
    role         VARCHAR(50) NOT NULL DEFAULT 'member',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_household_members_household
        FOREIGN KEY (household_id)
            REFERENCES households (id)
            ON DELETE CASCADE,

    CONSTRAINT fk_household_members_user
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE,

    CONSTRAINT uq_household_user
        UNIQUE (household_id, user_id)
);

CREATE INDEX idx_household_members_household_id
    ON household_members (household_id);

CREATE INDEX idx_household_members_user_id
    ON household_members (user_id);