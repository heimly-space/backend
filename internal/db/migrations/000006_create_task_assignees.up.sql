CREATE TABLE task_assignees
(
    id          UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    task_id     UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_task_assignees_task
        FOREIGN KEY (task_id)
            REFERENCES tasks (id)
            ON DELETE CASCADE,

    CONSTRAINT fk_task_assignees_user
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE,

    CONSTRAINT uq_task_user
        UNIQUE (task_id, user_id)
);

CREATE INDEX idx_task_assignees_task_id
    ON task_assignees (task_id);

CREATE INDEX idx_task_assignees_user_id
    ON task_assignees (user_id);