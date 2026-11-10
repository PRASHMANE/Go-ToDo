-- One server, one database + one user per service.
CREATE USER auth_user WITH PASSWORD 'auth_pass';
CREATE DATABASE auth_db OWNER auth_user;

CREATE USER todo_user WITH PASSWORD 'todo_pass';
CREATE DATABASE todo_db OWNER todo_user;

CREATE USER activity_user WITH PASSWORD 'activity_pass';
CREATE DATABASE activity_db OWNER activity_user;
