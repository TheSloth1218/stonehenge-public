create database stonehenge_vendor;
create user stonehenge_vendor with password 'change-me-local-dev';
grant all privileges on database stonehenge_vendor to stonehenge_vendor;
\connect stonehenge_vendor
grant usage, create on schema public to stonehenge_vendor;
alter default privileges in schema public grant all on tables to stonehenge_vendor;
alter default privileges in schema public grant all on sequences to stonehenge_vendor;
