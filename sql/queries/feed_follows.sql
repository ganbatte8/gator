
-- name: CreateFeedFollow :one
WITH inserted_feed_follow AS (
  INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id)
  VALUES ($1, NOW(), NOW(), $2, $3)
  RETURNING *
)
SELECT inserted_feed_follow.*, feeds.name AS feed_name, users.name AS user_name
FROM inserted_feed_follow
INNER JOIN users ON inserted_feed_follow.user_id = users.id
INNER JOIN feeds ON inserted_feed_follow.feed_id = feeds.id;


-- name: GetFeedFollowsForUser :many
SELECT feeds.name as feed_name FROM feed_follows
INNER JOIN users ON feed_follows.user_id = users.id
INNER JOIN feeds ON feed_follows.feed_id = feeds.id
WHERE feed_follows.user_id = $1;

-- name: DeleteFeedFollow :one
DELETE FROM feed_follows
USING feeds, users
WHERE feeds.id = feed_follows.feed_id AND users.id = feed_follows.user_id
AND feeds.url = $1 AND users.name = $2
RETURNING feed_follows.id;