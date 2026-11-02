create table shipment_jobs (
    id bigserial primary key,
    shipment_id bigint references shipments(id),
    fulfillment_order_id bigint references fulfillment_orders(id),
    maintenance_request_id bigint references maintenance_requests(id),
    external_request_id varchar(120),
    payload_format varchar(40) not null,
    encoded_payload text not null,
    status varchar(40) not null,
    attempt_count integer not null default 0 check (attempt_count >= 0),
    created_by_user_id bigint references users(id),
    replayed_by_user_id bigint references users(id),
    created_at timestamptz not null default now(),
    claimed_at timestamptz,
    processed_at timestamptz,
    last_error varchar(500),
    version bigint not null default 0,
    check (length(encoded_payload) <= 131072),
    check (status in ('QUEUED', 'CLAIMED', 'PROCESSING', 'COMPLETED', 'RETRYABLE_FAILED', 'PERMANENTLY_FAILED'))
);

create index idx_shipment_jobs_status_created on shipment_jobs(status, created_at);
create index idx_shipment_jobs_shipment on shipment_jobs(shipment_id);
create index idx_shipment_jobs_order on shipment_jobs(fulfillment_order_id);
