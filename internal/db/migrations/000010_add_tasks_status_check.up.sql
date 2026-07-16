ALTER TABLE tasks
    ADD CONSTRAINT chk_tasks_status
        CHECK (status IN ('pending', 'in_progress', 'done'));
