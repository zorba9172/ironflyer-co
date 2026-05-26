-- +goose Up
ALTER TABLE notifications
  ADD CONSTRAINT notifications_user_fk
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE notification_outbox
  ADD CONSTRAINT notification_outbox_user_fk
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE notification_outbox DROP CONSTRAINT IF EXISTS notification_outbox_user_fk;
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_user_fk;
