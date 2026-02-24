CREATE INDEX idx_household_members_user_created_household
    ON household_members (user_id, created_at DESC, household_id DESC);
