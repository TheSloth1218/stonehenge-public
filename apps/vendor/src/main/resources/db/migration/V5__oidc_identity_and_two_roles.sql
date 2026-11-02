alter table users add column dex_subject varchar(255);

update users
set role = case
    when role in ('FULFILLMENT_OPERATOR', 'WAREHOUSE_SUPERVISOR') then 'EMPLOYEE'
    when role = 'LOGISTICS_ADMIN' then 'ADMIN'
    else role
end;

alter table users add constraint users_role_check check (role in ('EMPLOYEE', 'ADMIN'));
create unique index ux_users_dex_subject on users(dex_subject) where dex_subject is not null;
