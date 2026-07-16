CREATE INDEX idx_tasks_household_created_id
    ON tasks (household_id, created_at DESC, id DESC);
