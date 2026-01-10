CREATE TABLE tasks
(
    id           UUID PRIMARY KEY      DEFAULT gen_random_uuid(),
    household_id UUID         NOT NULL,
    title        VARCHAR(255) NOT NULL,
    description  TEXT,
    status       VARCHAR(50)  NOT NULL DEFAULT 'pending',
    due_at       DATE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT fk_tasks_household
        FOREIGN KEY (household_id)
            REFERENCES households (id)
            ON DELETE CASCADE
);

CREATE INDEX idx_tasks_household_created_at
    ON tasks (household_id, created_at DESC);

CREATE INDEX idx_tasks_status
    ON tasks (status);