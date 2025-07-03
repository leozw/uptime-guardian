-- Drop tables in reverse order due to foreign key constraints
DROP TABLE IF EXISTS scheduled_checks CASCADE;
DROP TABLE IF EXISTS incidents CASCADE;
DROP TABLE IF EXISTS notification_channels CASCADE;
DROP TABLE IF EXISTS monitor_last_status CASCADE;
DROP TABLE IF EXISTS check_results CASCADE;
DROP TABLE IF EXISTS monitors CASCADE;

-- Drop extension
DROP EXTENSION IF EXISTS "uuid-ossp";