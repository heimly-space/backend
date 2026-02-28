CREATE INDEX idx_household_members_household_created_user
    ON household_members (household_id, created_at DESC, user_id DESC);
