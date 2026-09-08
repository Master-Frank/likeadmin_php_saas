-- Run as a MySQL administrator after replacing database/user/host.
-- CREATE/DROP/ALTER/INDEX are required by tactics=1 tenant provisioning and
-- online schema upgrades. LIKEADMIN_REQUIRE_DDL=1 verifies them at API start.
GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, ALTER, INDEX
ON `likeadmin_saas`.* TO 'likeadmin'@'127.0.0.1';
FLUSH PRIVILEGES;
