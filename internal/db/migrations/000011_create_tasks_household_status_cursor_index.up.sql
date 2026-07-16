CREATE INDEX idx_tasks_household_status_created_id
    ON tasks (household_id, status, created_at DESC, id DESC);
