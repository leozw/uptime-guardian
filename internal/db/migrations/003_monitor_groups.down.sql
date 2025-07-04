-- Drop tables in reverse order due to foreign key constraints
DROP TABLE IF EXISTS monitor_group_sla_reports CASCADE;
DROP TABLE IF EXISTS monitor_group_incidents CASCADE;
DROP TABLE IF EXISTS monitor_group_alert_rules CASCADE;
DROP TABLE IF EXISTS monitor_group_status CASCADE;
DROP TABLE IF EXISTS monitor_group_slos CASCADE;
DROP TABLE IF EXISTS monitor_group_members CASCADE;
DROP TABLE IF EXISTS monitor_groups CASCADE;